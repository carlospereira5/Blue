package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
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

// MatchSupplier busca en el comment de un CashMovement si coincide con algún
// alias de proveedor. Retorna el nombre del proveedor y true si hay match.
// La búsqueda es case-insensitive y por substring.
func MatchSupplier(comment string, suppliers map[string][]string) (string, bool) {
	lower := strings.ToLower(comment)
	for name, aliases := range suppliers {
		for _, alias := range aliases {
			if strings.Contains(lower, strings.ToLower(alias)) {
				return name, true
			}
		}
	}
	return "", false
}
