package tools

import (
	"encoding/json"
	"fmt"
	"os"
)

// LoadSuppliers carga el mapa de proveedores y sus aliases desde un archivo JSON.
// El formato esperado es: {"Nombre Proveedor": ["alias1", "alias2"]}.
func LoadSuppliers(path string) (map[string][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading suppliers file: %w", err)
	}
	var suppliers map[string][]string
	if err := json.Unmarshal(data, &suppliers); err != nil {
		return nil, fmt.Errorf("parsing suppliers JSON: %w", err)
	}
	return suppliers, nil
}
