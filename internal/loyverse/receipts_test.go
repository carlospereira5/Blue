package loyverse_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"aria/internal/loyverse"
)

func TestGetReceipts_Success(t *testing.T) {
	want := loyverse.ReceiptsResponse{
		Receipts: []loyverse.Receipt{
			{ReceiptNumber: "001", TotalMoney: 1500},
			{ReceiptNumber: "002", TotalMoney: 2000},
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
	if got.Receipts[0].ReceiptNumber != "001" {
		t.Errorf("receipts[0].ReceiptNumber = %q, want %q", got.Receipts[0].ReceiptNumber, "001")
	}
	if got.Receipts[1].TotalMoney != 2000 {
		t.Errorf("receipts[1].TotalMoney = %v, want 2000", got.Receipts[1].TotalMoney)
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

func TestGetReceipts_SendsDateParams(t *testing.T) {
	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("created_at_min") == "" {
			t.Error("missing created_at_min param")
		}
		if q.Get("created_at_max") == "" {
			t.Error("missing created_at_max param")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.ReceiptsResponse{}))
	})

	client.GetReceipts(context.Background(), time.Now().Add(-time.Hour), time.Now(), 10, "")
}

func TestGetAllReceipts_AutoPaginates(t *testing.T) {
	callCount := 0
	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Query().Get("cursor") {
		case "":
			w.Write(mustJSON(t, loyverse.ReceiptsResponse{
				Receipts: []loyverse.Receipt{{ReceiptNumber: "001"}, {ReceiptNumber: "002"}},
				Cursor:   "page2",
			}))
		case "page2":
			w.Write(mustJSON(t, loyverse.ReceiptsResponse{
				Receipts: []loyverse.Receipt{{ReceiptNumber: "003"}},
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
	if receipts[2].ReceiptNumber != "003" {
		t.Errorf("receipts[2].ReceiptNumber = %q, want %q", receipts[2].ReceiptNumber, "003")
	}
}

func TestGetAllReceipts_SinglePage(t *testing.T) {
	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.ReceiptsResponse{
			Receipts: []loyverse.Receipt{{ReceiptNumber: "001"}},
			Cursor:   "",
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

func TestGetReceiptByID_Success(t *testing.T) {
	want := loyverse.Receipt{ReceiptNumber: "001", TotalMoney: 1500}

	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/receipts/abc-123" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, want))
	})

	got, err := client.GetReceiptByID(context.Background(), "abc-123")
	if err != nil {
		t.Fatalf("GetReceiptByID() error = %v", err)
	}
	if got.ReceiptNumber != "001" {
		t.Errorf("ReceiptNumber = %q, want %q", got.ReceiptNumber, "001")
	}
}
