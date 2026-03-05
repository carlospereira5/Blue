package loyverse_test

import (
	"context"
	"net/http"
	"testing"

	"aria/internal/loyverse"
)

func TestGetSuppliers_Success(t *testing.T) {
	want := loyverse.SuppliersResponse{
		Suppliers: []loyverse.Supplier{
			{ID: "sup1", Name: "Distribuidora Norte"},
			{ID: "sup2", Name: "Lácteos Sur"},
		},
	}

	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/suppliers" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, want))
	})

	got, err := client.GetSuppliers(context.Background(), 50, "")
	if err != nil {
		t.Fatalf("GetSuppliers() error = %v", err)
	}
	if len(got.Suppliers) != 2 {
		t.Errorf("got %d suppliers, want 2", len(got.Suppliers))
	}
	if got.Suppliers[0].Name != "Distribuidora Norte" {
		t.Errorf("Suppliers[0].Name = %q, want %q", got.Suppliers[0].Name, "Distribuidora Norte")
	}
}

func TestGetSupplierByID_Success(t *testing.T) {
	want := loyverse.Supplier{ID: "sup1", Name: "Distribuidora Norte"}

	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/suppliers/sup1" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, want))
	})

	got, err := client.GetSupplierByID(context.Background(), "sup1")
	if err != nil {
		t.Fatalf("GetSupplierByID() error = %v", err)
	}
	if got.Name != "Distribuidora Norte" {
		t.Errorf("Name = %q, want %q", got.Name, "Distribuidora Norte")
	}
}

func TestGetAllSuppliers_AutoPaginates(t *testing.T) {
	callCount := 0
	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Query().Get("cursor") {
		case "":
			w.Write(mustJSON(t, loyverse.SuppliersResponse{
				Suppliers: []loyverse.Supplier{{ID: "sup1"}, {ID: "sup2"}},
				Cursor:    "page2",
			}))
		case "page2":
			w.Write(mustJSON(t, loyverse.SuppliersResponse{
				Suppliers: []loyverse.Supplier{{ID: "sup3"}},
				Cursor:    "",
			}))
		default:
			http.Error(w, "unexpected cursor", http.StatusBadRequest)
		}
	})

	suppliers, err := client.GetAllSuppliers(context.Background())
	if err != nil {
		t.Fatalf("GetAllSuppliers() error = %v", err)
	}
	if len(suppliers) != 3 {
		t.Errorf("got %d suppliers, want 3", len(suppliers))
	}
	if callCount != 2 {
		t.Errorf("made %d API calls, want 2", callCount)
	}
}
