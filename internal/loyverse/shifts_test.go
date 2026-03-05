package loyverse_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"aria/internal/loyverse"
)

func TestGetShifts_UsesCreatedAtParams(t *testing.T) {
	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("created_at_min") == "" {
			t.Error("missing created_at_min — GetShifts should use created_at_min, not opened_at_min")
		}
		if q.Get("created_at_max") == "" {
			t.Error("missing created_at_max — GetShifts should use created_at_max, not opened_at_max")
		}
		if q.Get("opened_at_min") != "" {
			t.Error("found opened_at_min — this is the old buggy param")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.ShiftsResponse{}))
	})

	client.GetShifts(context.Background(), time.Now().Add(-time.Hour), time.Now(), 10, "")
}

func TestGetShifts_Success(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	want := loyverse.ShiftsResponse{
		Shifts: []loyverse.Shift{
			{
				ID:           "s1",
				StartingCash: 5000,
				GrossSales:   15000,
				NetSales:     14000,
				CashMovements: []loyverse.CashMovement{
					{Type: "PAY_OUT", MoneyAmount: 2000, Comment: "Proveedor leche"},
				},
				Payments: []loyverse.ShiftPayment{
					{PaymentTypeID: "pt1", MoneyAmount: 10000},
				},
				OpenedAt: now,
			},
		},
	}

	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, want))
	})

	got, err := client.GetShifts(context.Background(), time.Now().Add(-24*time.Hour), time.Now(), 10, "")
	if err != nil {
		t.Fatalf("GetShifts() error = %v", err)
	}
	if len(got.Shifts) != 1 {
		t.Fatalf("got %d shifts, want 1", len(got.Shifts))
	}
	s := got.Shifts[0]
	if s.GrossSales != 15000 {
		t.Errorf("GrossSales = %v, want 15000", s.GrossSales)
	}
	if len(s.CashMovements) != 1 {
		t.Fatalf("got %d cash_movements, want 1", len(s.CashMovements))
	}
	if s.CashMovements[0].Comment != "Proveedor leche" {
		t.Errorf("CashMovement.Comment = %q, want %q", s.CashMovements[0].Comment, "Proveedor leche")
	}
	if len(s.Payments) != 1 {
		t.Fatalf("got %d payments, want 1", len(s.Payments))
	}
	if s.Payments[0].MoneyAmount != 10000 {
		t.Errorf("Payment.MoneyAmount = %v, want 10000", s.Payments[0].MoneyAmount)
	}
}

func TestGetShiftByID_Success(t *testing.T) {
	want := loyverse.Shift{
		ID:       "s1",
		NetSales: 14000,
		CashMovements: []loyverse.CashMovement{
			{Type: "PAY_IN", MoneyAmount: 1000, Comment: "Cambio"},
		},
	}

	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/shifts/s1" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, want))
	})

	got, err := client.GetShiftByID(context.Background(), "s1")
	if err != nil {
		t.Fatalf("GetShiftByID() error = %v", err)
	}
	if got.ID != "s1" {
		t.Errorf("ID = %q, want %q", got.ID, "s1")
	}
	if got.NetSales != 14000 {
		t.Errorf("NetSales = %v, want 14000", got.NetSales)
	}
}

func TestGetAllShifts_AutoPaginates(t *testing.T) {
	callCount := 0
	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Query().Get("cursor") {
		case "":
			w.Write(mustJSON(t, loyverse.ShiftsResponse{
				Shifts: []loyverse.Shift{{ID: "s1"}, {ID: "s2"}},
				Cursor: "page2",
			}))
		case "page2":
			w.Write(mustJSON(t, loyverse.ShiftsResponse{
				Shifts: []loyverse.Shift{{ID: "s3"}},
				Cursor: "",
			}))
		default:
			http.Error(w, "unexpected cursor", http.StatusBadRequest)
		}
	})

	shifts, err := client.GetAllShifts(context.Background(), time.Now().Add(-24*time.Hour), time.Now())
	if err != nil {
		t.Fatalf("GetAllShifts() error = %v", err)
	}
	if len(shifts) != 3 {
		t.Errorf("got %d shifts, want 3", len(shifts))
	}
	if callCount != 2 {
		t.Errorf("made %d API calls, want 2", callCount)
	}
}
