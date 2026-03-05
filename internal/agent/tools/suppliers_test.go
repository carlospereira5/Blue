package tools_test

import (
	"os"
	"path/filepath"
	"testing"

	"aria/internal/agent/tools"
)

func TestLoadSuppliers(t *testing.T) {
	t.Run("valid JSON", func(t *testing.T) {
		path := writeTempJSON(t, `{
			"Coca Cola": ["coca", "coca-cola"],
			"Lácteos Sur": ["lacteos", "leche"]
		}`)

		suppliers, err := tools.LoadSuppliers(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(suppliers) != 2 {
			t.Fatalf("want 2 suppliers, got %d", len(suppliers))
		}
		if len(suppliers["Coca Cola"]) != 2 {
			t.Errorf("want 2 aliases for Coca Cola, got %d", len(suppliers["Coca Cola"]))
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := tools.LoadSuppliers("/nonexistent/suppliers.json")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		path := writeTempJSON(t, `{invalid}`)
		_, err := tools.LoadSuppliers(path)
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

func writeTempJSON(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "suppliers.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	return path
}
