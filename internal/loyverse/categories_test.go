package loyverse_test

import (
	"context"
	"net/http"
	"testing"

	"blue/internal/loyverse"
)

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
