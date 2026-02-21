package loyverse_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"blue/internal/loyverse"
)

// newTestClient crea un cliente apuntando a un servidor de test.
func newTestClient(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *loyverse.Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := loyverse.NewClient(srv.Client(), "test-token", loyverse.WithBaseURL(srv.URL))
	return srv, client
}

// mustJSON serializa v a JSON o falla el test.
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	return b
}

// --- GetReceipts ---

func TestGetReceipts_Success(t *testing.T) {
	want := loyverse.ReceiptsResponse{
		Receipts: []loyverse.Receipt{
			{ID: "r1", ReceiptNumber: "001", ReceiptTotal: 1500},
			{ID: "r2", ReceiptNumber: "002", ReceiptTotal: 2000},
		},
		Cursor: "",
	}

	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/receipts" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, want))
	})

	since := time.Now().Add(-24 * time.Hour)
	until := time.Now()
	got, err := client.GetReceipts(context.Background(), since, until, 10, "")
	if err != nil {
		t.Fatalf("GetReceipts() error = %v", err)
	}
	if len(got.Receipts) != 2 {
		t.Errorf("got %d receipts, want 2", len(got.Receipts))
	}
	if got.Receipts[0].ID != "r1" {
		t.Errorf("receipts[0].ID = %q, want %q", got.Receipts[0].ID, "r1")
	}
	if got.Receipts[1].ReceiptTotal != 2000 {
		t.Errorf("receipts[1].ReceiptTotal = %v, want 2000", got.Receipts[1].ReceiptTotal)
	}
}

func TestGetReceipts_HTTPError(t *testing.T) {
	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"invalid_token"}`, http.StatusUnauthorized)
	})

	_, err := client.GetReceipts(context.Background(), time.Now().Add(-time.Hour), time.Now(), 10, "")
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
}

func TestGetReceipts_SendsAuthHeader(t *testing.T) {
	var capturedAuth string
	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.ReceiptsResponse{}))
	})

	client.GetReceipts(context.Background(), time.Now().Add(-time.Hour), time.Now(), 10, "")

	if capturedAuth != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", capturedAuth, "Bearer test-token")
	}
}

// --- GetAllReceipts (pagination) ---

func TestGetAllReceipts_AutoPaginates(t *testing.T) {
	callCount := 0
	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Query().Get("cursor") {
		case "":
			w.Write(mustJSON(t, loyverse.ReceiptsResponse{
				Receipts: []loyverse.Receipt{{ID: "r1"}, {ID: "r2"}},
				Cursor:   "page2",
			}))
		case "page2":
			w.Write(mustJSON(t, loyverse.ReceiptsResponse{
				Receipts: []loyverse.Receipt{{ID: "r3"}},
				Cursor:   "",
			}))
		default:
			http.Error(w, "unexpected cursor", http.StatusBadRequest)
		}
	})

	receipts, err := client.GetAllReceipts(context.Background(), time.Now().Add(-24*time.Hour), time.Now())
	if err != nil {
		t.Fatalf("GetAllReceipts() error = %v", err)
	}
	if len(receipts) != 3 {
		t.Errorf("got %d receipts, want 3", len(receipts))
	}
	if callCount != 2 {
		t.Errorf("made %d API calls, want 2", callCount)
	}
	if receipts[2].ID != "r3" {
		t.Errorf("receipts[2].ID = %q, want %q", receipts[2].ID, "r3")
	}
}

func TestGetAllReceipts_SinglePage(t *testing.T) {
	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.ReceiptsResponse{
			Receipts: []loyverse.Receipt{{ID: "r1"}},
			Cursor:   "", // no next page
		}))
	})

	receipts, err := client.GetAllReceipts(context.Background(), time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(receipts) != 1 {
		t.Errorf("got %d receipts, want 1", len(receipts))
	}
}

// --- GetItems ---

func TestGetItems_Success(t *testing.T) {
	want := loyverse.ItemsResponse{
		Items: []loyverse.Item{
			{ID: "i1", ItemName: "Coca Cola", Variants: []loyverse.Variation{{Price: 800}}},
		},
	}

	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/items" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, want))
	})

	got, err := client.GetItems(context.Background(), 10, "")
	if err != nil {
		t.Fatalf("GetItems() error = %v", err)
	}
	if len(got.Items) != 1 {
		t.Errorf("got %d items, want 1", len(got.Items))
	}
	if got.Items[0].EffectivePrice() != 800 {
		t.Errorf("EffectivePrice() = %v, want 800", got.Items[0].EffectivePrice())
	}
}

// --- GetCategories ---

func TestGetCategories_Success(t *testing.T) {
	want := loyverse.CategoriesResponse{
		Categories: []loyverse.Category{
			{ID: "c1", Name: "Bebidas"},
			{ID: "c2", Name: "Snacks"},
		},
	}

	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, want))
	})

	got, err := client.GetCategories(context.Background())
	if err != nil {
		t.Fatalf("GetCategories() error = %v", err)
	}
	if len(got.Categories) != 2 {
		t.Errorf("got %d categories, want 2", len(got.Categories))
	}
}

// --- EffectivePrice ---

func TestEffectivePrice_WithVariants(t *testing.T) {
	item := loyverse.Item{
		Price:    500,
		Variants: []loyverse.Variation{{Price: 800}},
	}
	if got := item.EffectivePrice(); got != 800 {
		t.Errorf("EffectivePrice() = %v, want 800 (should use variant price)", got)
	}
}

func TestEffectivePrice_NoVariants(t *testing.T) {
	item := loyverse.Item{Price: 500}
	if got := item.EffectivePrice(); got != 500 {
		t.Errorf("EffectivePrice() = %v, want 500 (should fall back to item price)", got)
	}
}

// --- SortItems ---

func TestSortItems_ByNameAsc(t *testing.T) {
	items := []loyverse.Item{
		{ItemName: "Zebra"},
		{ItemName: "Apple"},
		{ItemName: "Mango"},
	}

	loyverse.SortItems(items, loyverse.SortByName, loyverse.SortAsc)

	names := []string{items[0].ItemName, items[1].ItemName, items[2].ItemName}
	want := []string{"Apple", "Mango", "Zebra"}
	for i, name := range names {
		if name != want[i] {
			t.Errorf("items[%d].ItemName = %q, want %q", i, name, want[i])
		}
	}
}

func TestSortItems_ByNameDesc(t *testing.T) {
	items := []loyverse.Item{
		{ItemName: "Apple"},
		{ItemName: "Zebra"},
		{ItemName: "Mango"},
	}

	loyverse.SortItems(items, loyverse.SortByName, loyverse.SortDesc)

	if items[0].ItemName != "Zebra" {
		t.Errorf("items[0].ItemName = %q, want %q", items[0].ItemName, "Zebra")
	}
	if items[2].ItemName != "Apple" {
		t.Errorf("items[2].ItemName = %q, want %q", items[2].ItemName, "Apple")
	}
}

func TestSortItems_ByPrice(t *testing.T) {
	items := []loyverse.Item{
		{ItemName: "Expensive", Variants: []loyverse.Variation{{Price: 1000}}},
		{ItemName: "Cheap", Variants: []loyverse.Variation{{Price: 100}}},
		{ItemName: "Mid", Variants: []loyverse.Variation{{Price: 500}}},
	}

	loyverse.SortItems(items, loyverse.SortByPrice, loyverse.SortAsc)

	if items[0].ItemName != "Cheap" {
		t.Errorf("items[0] = %q, want Cheap", items[0].ItemName)
	}
	if items[2].ItemName != "Expensive" {
		t.Errorf("items[2] = %q, want Expensive", items[2].ItemName)
	}
}

// --- SortReceipts ---

func TestSortReceipts_ByTotalAsc(t *testing.T) {
	receipts := []loyverse.Receipt{
		{ID: "r1", ReceiptTotal: 3000},
		{ID: "r2", ReceiptTotal: 1000},
		{ID: "r3", ReceiptTotal: 2000},
	}

	loyverse.SortReceipts(receipts, loyverse.SortByTotal, loyverse.SortAsc)

	if receipts[0].ReceiptTotal != 1000 {
		t.Errorf("receipts[0].ReceiptTotal = %v, want 1000", receipts[0].ReceiptTotal)
	}
	if receipts[2].ReceiptTotal != 3000 {
		t.Errorf("receipts[2].ReceiptTotal = %v, want 3000", receipts[2].ReceiptTotal)
	}
}

func TestSortReceipts_ByDate(t *testing.T) {
	now := time.Now()
	receipts := []loyverse.Receipt{
		{ID: "newest", CreatedAt: now},
		{ID: "oldest", CreatedAt: now.Add(-48 * time.Hour)},
		{ID: "middle", CreatedAt: now.Add(-24 * time.Hour)},
	}

	loyverse.SortReceipts(receipts, loyverse.SortByDate, loyverse.SortAsc)

	if receipts[0].ID != "oldest" {
		t.Errorf("receipts[0].ID = %q, want oldest", receipts[0].ID)
	}
	if receipts[2].ID != "newest" {
		t.Errorf("receipts[2].ID = %q, want newest", receipts[2].ID)
	}
}

// --- SortCategories ---

func TestSortCategories_Asc(t *testing.T) {
	cats := []loyverse.Category{
		{Name: "Snacks"},
		{Name: "Bebidas"},
		{Name: "Limpieza"},
	}

	loyverse.SortCategories(cats, loyverse.SortAsc)

	if cats[0].Name != "Bebidas" {
		t.Errorf("cats[0].Name = %q, want Bebidas", cats[0].Name)
	}
}
