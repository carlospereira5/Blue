package cortex

import (
	"sort"
	"strings"

	"aria/internal/loyverse"
)

// SearchMatch representa un resultado de búsqueda fuzzy con su score de confianza.
type SearchMatch struct {
	EntityID      string
	CanonicalName string
	Score         float64 // 1.0=exacto, 0.9=prefijo, 0.7=contiene
}

// nameIDPair es una estructura interna para búsqueda genérica por nombre.
type nameIDPair struct {
	ID   string
	Name string
}

// searchByName aplica three-tier fuzzy scoring sobre un slice de pares nombre/ID.
// Scores: exact=1.0, prefix=0.9, contains=0.7. Sin match → excluido.
// Resultado ordenado: score desc, luego nombre asc (orden estable).
// Es una función PURA: no hace I/O, no accede a red ni DB.
func searchByName(pairs []nameIDPair, query string, maxResults int) []SearchMatch {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}

	var matches []SearchMatch
	for _, p := range pairs {
		name := strings.ToLower(p.Name)
		var score float64
		switch {
		case name == q:
			score = 1.0
		case strings.HasPrefix(name, q):
			score = 0.9
		case strings.Contains(name, q):
			score = 0.7
		default:
			continue
		}
		matches = append(matches, SearchMatch{
			EntityID:      p.ID,
			CanonicalName: p.Name,
			Score:         score,
		})
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		return matches[i].CanonicalName < matches[j].CanonicalName
	})

	if maxResults > 0 && len(matches) > maxResults {
		matches = matches[:maxResults]
	}
	return matches
}

// SearchItems busca productos por nombre usando fuzzy matching three-tier.
// Diseñado para el handler search_product: recibe el catálogo completo de items.
func SearchItems(items []loyverse.Item, query string, maxResults int) []SearchMatch {
	pairs := make([]nameIDPair, len(items))
	for i, it := range items {
		pairs[i] = nameIDPair{ID: it.ID, Name: it.ItemName}
	}
	return searchByName(pairs, query, maxResults)
}

// SearchCategories busca categorías por nombre usando fuzzy matching three-tier.
func SearchCategories(cats []loyverse.Category, query string, maxResults int) []SearchMatch {
	pairs := make([]nameIDPair, len(cats))
	for i, c := range cats {
		pairs[i] = nameIDPair{ID: c.ID, Name: c.Name}
	}
	return searchByName(pairs, query, maxResults)
}

// SearchEmployees busca empleados por nombre usando fuzzy matching three-tier.
func SearchEmployees(emps []loyverse.Employee, query string, maxResults int) []SearchMatch {
	pairs := make([]nameIDPair, len(emps))
	for i, e := range emps {
		pairs[i] = nameIDPair{ID: e.ID, Name: e.Name}
	}
	return searchByName(pairs, query, maxResults)
}
