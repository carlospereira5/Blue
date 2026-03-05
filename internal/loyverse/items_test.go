package loyverse_test

import (
	"context"
	"net/http"
	"testing"

	"aria/internal/loyverse"
)

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

func TestGetItemByID_Success(t *testing.T) {
	want := loyverse.Item{ID: "i1", ItemName: "Coca Cola", Price: 500}

	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/items/i1" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, want))
	})

	got, err := client.GetItemByID(context.Background(), "i1")
	if err != nil {
		t.Fatalf("GetItemByID() error = %v", err)
	}
	if got.ItemName != "Coca Cola" {
		t.Errorf("ItemName = %q, want %q", got.ItemName, "Coca Cola")
	}
}

func TestGetAllItems_AutoPaginates(t *testing.T) {
	callCount := 0
	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Query().Get("cursor") {
		case "":
			w.Write(mustJSON(t, loyverse.ItemsResponse{
				Items:  []loyverse.Item{{ID: "i1"}, {ID: "i2"}},
				Cursor: "page2",
			}))
		case "page2":
			w.Write(mustJSON(t, loyverse.ItemsResponse{
				Items:  []loyverse.Item{{ID: "i3"}},
				Cursor: "",
			}))
		default:
			http.Error(w, "unexpected cursor", http.StatusBadRequest)
		}
	})

	items, err := client.GetAllItems(context.Background())
	if err != nil {
		t.Fatalf("GetAllItems() error = %v", err)
	}
	if len(items) != 3 {
		t.Errorf("got %d items, want 3", len(items))
	}
	if callCount != 2 {
		t.Errorf("made %d API calls, want 2", callCount)
	}
}

func TestItemNameToID_Success(t *testing.T) {
	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.ItemsResponse{
			Items: []loyverse.Item{
				{ID: "i1", ItemName: "Coca Cola"},
				{ID: "i2", ItemName: "Pepsi"},
			},
		}))
	})

	result, err := client.ItemNameToID(context.Background())
	if err != nil {
		t.Fatalf("ItemNameToID() error = %v", err)
	}
	if result["Coca Cola"] != "i1" {
		t.Errorf("result[\"Coca Cola\"] = %q, want %q", result["Coca Cola"], "i1")
	}
	if result["coca cola"] != "i1" {
		t.Errorf("lowercase lookup failed: result[\"coca cola\"] = %q, want %q", result["coca cola"], "i1")
	}
}

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
