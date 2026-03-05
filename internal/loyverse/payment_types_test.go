package loyverse_test

import (
	"context"
	"net/http"
	"testing"

	"aria/internal/loyverse"
)

func TestGetPaymentTypes_Success(t *testing.T) {
	want := loyverse.PaymentTypesResponse{
		PaymentTypes: []loyverse.PaymentType{
			{ID: "pt1", Name: "Efectivo", Type: "CASH"},
			{ID: "pt2", Name: "Tarjeta", Type: "CARD"},
		},
	}

	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/payment_types" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, want))
	})

	got, err := client.GetPaymentTypes(context.Background())
	if err != nil {
		t.Fatalf("GetPaymentTypes() error = %v", err)
	}
	if len(got.PaymentTypes) != 2 {
		t.Errorf("got %d payment types, want 2", len(got.PaymentTypes))
	}
	if got.PaymentTypes[0].Name != "Efectivo" {
		t.Errorf("PaymentTypes[0].Name = %q, want %q", got.PaymentTypes[0].Name, "Efectivo")
	}
}

func TestGetPaymentTypeByID_Success(t *testing.T) {
	want := loyverse.PaymentType{ID: "pt1", Name: "Efectivo", Type: "CASH"}

	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/payment_types/pt1" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, want))
	})

	got, err := client.GetPaymentTypeByID(context.Background(), "pt1")
	if err != nil {
		t.Fatalf("GetPaymentTypeByID() error = %v", err)
	}
	if got.Name != "Efectivo" {
		t.Errorf("Name = %q, want %q", got.Name, "Efectivo")
	}
}
