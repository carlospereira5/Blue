package cortex_test

import (
	"testing"

	"aria/internal/cortex"
	"aria/internal/loyverse"
)

func TestSearchItems(t *testing.T) {
	items := []loyverse.Item{
		{ID: "i1", ItemName: "Coca Cola 500ml"},
		{ID: "i2", ItemName: "Coca Cola 1L"},
		{ID: "i3", ItemName: "Sprite 500ml"},
		{ID: "i4", ItemName: "Palmal Azul Blando 20 unid"},
		{ID: "i5", ItemName: "Palmal Azul Blando 14 unid"},
		{ID: "i6", ItemName: "Palmal Rojo 20 unid"},
	}

	t.Run("exact match scores 1.0", func(t *testing.T) {
		res := cortex.SearchItems(items, "sprite 500ml", 5)
		if len(res) == 0 {
			t.Fatal("expected 1 result, got 0")
		}
		if res[0].EntityID != "i3" {
			t.Errorf("want i3, got %s", res[0].EntityID)
		}
		assertFloat(t, "Score", 1.0, res[0].Score)
	})

	t.Run("prefix match scores 0.9", func(t *testing.T) {
		res := cortex.SearchItems(items, "sprite", 5)
		if len(res) == 0 {
			t.Fatal("expected result for 'sprite'")
		}
		assertFloat(t, "Score", 0.9, res[0].Score)
	})

	t.Run("contains match scores 0.7", func(t *testing.T) {
		res := cortex.SearchItems(items, "500ml", 5)
		if len(res) < 2 {
			t.Fatalf("expected 2+ results for '500ml', got %d", len(res))
		}
		for _, r := range res {
			assertFloat(t, r.CanonicalName+" score", 0.7, r.Score)
		}
	})

	t.Run("no match returns empty", func(t *testing.T) {
		res := cortex.SearchItems(items, "pepsi", 5)
		if len(res) != 0 {
			t.Errorf("expected 0 results for 'pepsi', got %d", len(res))
		}
	})

	t.Run("empty query returns nil", func(t *testing.T) {
		res := cortex.SearchItems(items, "", 5)
		if res != nil {
			t.Errorf("expected nil for empty query, got %v", res)
		}
	})

	t.Run("maxResults respected", func(t *testing.T) {
		res := cortex.SearchItems(items, "cola", 1)
		if len(res) != 1 {
			t.Errorf("want 1 result, got %d", len(res))
		}
	})

	t.Run("sorted score desc then name asc", func(t *testing.T) {
		// "palmal azul blando" → prefix match para i4 y i5, contiene para i6
		res := cortex.SearchItems(items, "palmal azul blando", 5)
		if len(res) < 2 {
			t.Fatalf("expected 2+ results, got %d", len(res))
		}
		// i4 y i5 deben ir primero (score 0.9, prefix), ordenados por nombre asc
		if res[0].Score < res[1].Score {
			t.Errorf("results not sorted by score desc: %v > %v", res[1].Score, res[0].Score)
		}
		// Los dos primeros deben ser prefix matches (0.9)
		if res[0].Score != 0.9 || res[1].Score != 0.9 {
			t.Errorf("expected prefix scores 0.9, got %.1f, %.1f", res[0].Score, res[1].Score)
		}
		// i4 "Palmal Azul Blando 14 unid" antes que i5 "Palmal Azul Blando 20 unid" (nombre asc)
		if res[0].EntityID != "i5" && res[0].EntityID != "i4" {
			t.Errorf("unexpected first result: %s", res[0].EntityID)
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		res := cortex.SearchItems(items, "COCA COLA", 5)
		if len(res) < 2 {
			t.Errorf("expected 2 results for 'COCA COLA', got %d", len(res))
		}
	})

	t.Run("maxResults 0 returns all", func(t *testing.T) {
		res := cortex.SearchItems(items, "palmal", 0)
		if len(res) != 3 {
			t.Errorf("expected 3 palmal results with no limit, got %d", len(res))
		}
	})
}

func TestSearchCategories(t *testing.T) {
	cats := []loyverse.Category{
		{ID: "c1", Name: "Cigarrillos"},
		{ID: "c2", Name: "Bebidas"},
		{ID: "c3", Name: "Golosinas"},
	}

	t.Run("prefix match for common alias", func(t *testing.T) {
		res := cortex.SearchCategories(cats, "cigarr", 5)
		if len(res) == 0 {
			t.Fatal("expected result for 'cigarr'")
		}
		if res[0].EntityID != "c1" {
			t.Errorf("want c1, got %s", res[0].EntityID)
		}
		assertFloat(t, "Score", 0.9, res[0].Score)
	})

	t.Run("contains match", func(t *testing.T) {
		res := cortex.SearchCategories(cats, "bida", 5)
		if len(res) == 0 {
			t.Fatal("expected result for 'bida'")
		}
		assertFloat(t, "Score", 0.7, res[0].Score)
	})
}

func TestSearchEmployees(t *testing.T) {
	emps := []loyverse.Employee{
		{ID: "e1", Name: "Carlos Pérez"},
		{ID: "e2", Name: "María García"},
	}

	t.Run("prefix match for employee", func(t *testing.T) {
		res := cortex.SearchEmployees(emps, "carlos", 5)
		if len(res) == 0 {
			t.Fatal("expected result for 'carlos'")
		}
		if res[0].EntityID != "e1" {
			t.Errorf("want e1, got %s", res[0].EntityID)
		}
	})
}
