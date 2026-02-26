package loyverse_test

import (
	"context"
	"net/http"
	"testing"

	"blue/internal/loyverse"
)

func TestGetStores_Success(t *testing.T) {
	want := loyverse.StoresResponse{
		Stores: []loyverse.Store{
			{ID: "st1", Name: "Kiosco Centro", Address: "Av. Corrientes 1234"},
		},
	}

	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stores" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, want))
	})

	got, err := client.GetStores(context.Background())
	if err != nil {
		t.Fatalf("GetStores() error = %v", err)
	}
	if len(got.Stores) != 1 {
		t.Errorf("got %d stores, want 1", len(got.Stores))
	}
	if got.Stores[0].Name != "Kiosco Centro" {
		t.Errorf("Stores[0].Name = %q, want %q", got.Stores[0].Name, "Kiosco Centro")
	}
}

func TestGetStoreByID_Success(t *testing.T) {
	want := loyverse.Store{ID: "st1", Name: "Kiosco Centro"}

	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stores/st1" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, want))
	})

	got, err := client.GetStoreByID(context.Background(), "st1")
	if err != nil {
		t.Fatalf("GetStoreByID() error = %v", err)
	}
	if got.Name != "Kiosco Centro" {
		t.Errorf("Name = %q, want %q", got.Name, "Kiosco Centro")
	}
}
