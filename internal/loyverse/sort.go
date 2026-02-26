package loyverse

import (
	"sort"
	"strings"
)

// SortOption y SortOrder controlan el ordenamiento de listas.
type (
	SortOption string
	SortOrder  string
)

const (
	SortByName       SortOption = "name"
	SortByCategory   SortOption = "category"
	SortByPrice      SortOption = "price"
	SortByDate       SortOption = "date"
	SortByTotal      SortOption = "total"
	SortByReceiptNum SortOption = "receipt_number"
	SortAsc          SortOrder  = "asc"
	SortDesc         SortOrder  = "desc"
)

// reverse invierte un slice in-place.
func reverse[S ~[]E, E any](s S) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// SortItems ordena items por el criterio y orden especificados.
func SortItems(items []Item, by SortOption, order SortOrder) {
	switch by {
	case SortByName:
		sort.Slice(items, func(i, j int) bool {
			return strings.ToLower(items[i].ItemName) < strings.ToLower(items[j].ItemName)
		})
	case SortByPrice:
		sort.Slice(items, func(i, j int) bool {
			return items[i].EffectivePrice() < items[j].EffectivePrice()
		})
	case SortByCategory:
		sort.Slice(items, func(i, j int) bool {
			return items[i].CategoryID < items[j].CategoryID
		})
	}
	if order == SortDesc {
		reverse(items)
	}
}

// SortReceipts ordena receipts por el criterio y orden especificados.
func SortReceipts(receipts []Receipt, by SortOption, order SortOrder) {
	switch by {
	case SortByDate:
		sort.Slice(receipts, func(i, j int) bool {
			return receipts[i].CreatedAt.Before(receipts[j].CreatedAt)
		})
	case SortByTotal:
		sort.Slice(receipts, func(i, j int) bool {
			return receipts[i].TotalMoney < receipts[j].TotalMoney
		})
	case SortByReceiptNum:
		sort.Slice(receipts, func(i, j int) bool {
			return receipts[i].ReceiptNumber < receipts[j].ReceiptNumber
		})
	}
	if order == SortDesc {
		reverse(receipts)
	}
}

// SortCategories ordena categorías alfabéticamente.
func SortCategories(categories []Category, order SortOrder) {
	sort.Slice(categories, func(i, j int) bool {
		return strings.ToLower(categories[i].Name) < strings.ToLower(categories[j].Name)
	})
	if order == SortDesc {
		reverse(categories)
	}
}
