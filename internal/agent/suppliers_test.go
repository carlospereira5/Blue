package agent_test

import (
	"os"
	"path/filepath"
	"testing"

	"aria/internal/agent"
)

func TestLoadSuppliers(t *testing.T) {
	t.Run("valid JSON", func(t *testing.T) {
		path := writeTempJSON(t, `{
			"Coca Cola": ["coca", "coca-cola"],
			"Lácteos Sur": ["lacteos", "leche"]
		}`)

		suppliers, err := agent.LoadSuppliers(path)
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
		_, err := agent.LoadSuppliers("/nonexistent/suppliers.json")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		path := writeTempJSON(t, `{invalid}`)
		_, err := agent.LoadSuppliers(path)
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

func TestMatchSupplier(t *testing.T) {
	suppliers := map[string][]string{
		"Coca Cola":          {"coca", "coca-cola"},
		"Distribuidora Norte": {"distribuidora", "dist norte"},
		"Lácteos Sur":        {"lacteos", "leche"},
	}

	tests := []struct {
		name      string
		comment   string
		wantName  string
		wantMatch bool
	}{
		{
			name:      "exact alias match",
			comment:   "coca",
			wantName:  "Coca Cola",
			wantMatch: true,
		},
		{
			name:      "case insensitive",
			comment:   "COCA-COLA pago mensual",
			wantName:  "Coca Cola",
			wantMatch: true,
		},
		{
			name:      "substring match",
			comment:   "Pago distribuidora factura 123",
			wantName:  "Distribuidora Norte",
			wantMatch: true,
		},
		{
			name:      "accent insensitive alias",
			comment:   "Pago lacteos del mes",
			wantName:  "Lácteos Sur",
			wantMatch: true,
		},
		{
			name:      "no match",
			comment:   "Compra de insumos varios",
			wantName:  "",
			wantMatch: false,
		},
		{
			name:      "empty comment",
			comment:   "",
			wantName:  "",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, matched := agent.MatchSupplier(tt.comment, suppliers)
			if matched != tt.wantMatch {
				t.Errorf("matched = %v, want %v", matched, tt.wantMatch)
			}
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
		})
	}
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
