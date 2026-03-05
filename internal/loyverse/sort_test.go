package loyverse_test

import (
	"testing"
	"time"

	"aria/internal/loyverse"
)

func TestSortItems_ByNameAsc(t *testing.T) {
	items := []loyverse.Item{
		{ItemName: "Zebra"},
		{ItemName: "Apple"},
		{ItemName: "Mango"},
	}

	loyverse.SortItems(items, loyverse.SortByName, loyverse.SortAsc)

	names := []string{items[0].ItemName, items[1].ItemName, items[2].ItemName}
	want := []string{"Apple", "Mango", "Zebra"}
	for i, name := range names {
		if name != want[i] {
			t.Errorf("items[%d].ItemName = %q, want %q", i, name, want[i])
		}
	}
}

func TestSortItems_ByNameDesc(t *testing.T) {
	items := []loyverse.Item{
		{ItemName: "Apple"},
		{ItemName: "Zebra"},
		{ItemName: "Mango"},
	}

	loyverse.SortItems(items, loyverse.SortByName, loyverse.SortDesc)

	if items[0].ItemName != "Zebra" {
		t.Errorf("items[0].ItemName = %q, want %q", items[0].ItemName, "Zebra")
	}
	if items[2].ItemName != "Apple" {
		t.Errorf("items[2].ItemName = %q, want %q", items[2].ItemName, "Apple")
	}
}

func TestSortItems_ByPrice(t *testing.T) {
	items := []loyverse.Item{
		{ItemName: "Expensive", Variants: []loyverse.Variation{{Price: 1000}}},
		{ItemName: "Cheap", Variants: []loyverse.Variation{{Price: 100}}},
		{ItemName: "Mid", Variants: []loyverse.Variation{{Price: 500}}},
	}

	loyverse.SortItems(items, loyverse.SortByPrice, loyverse.SortAsc)

	if items[0].ItemName != "Cheap" {
		t.Errorf("items[0] = %q, want Cheap", items[0].ItemName)
	}
	if items[2].ItemName != "Expensive" {
		t.Errorf("items[2] = %q, want Expensive", items[2].ItemName)
	}
}

func TestSortReceipts_ByTotalAsc(t *testing.T) {
	receipts := []loyverse.Receipt{
		{ReceiptNumber: "r1", TotalMoney: 3000},
		{ReceiptNumber: "r2", TotalMoney: 1000},
		{ReceiptNumber: "r3", TotalMoney: 2000},
	}

	loyverse.SortReceipts(receipts, loyverse.SortByTotal, loyverse.SortAsc)

	if receipts[0].TotalMoney != 1000 {
		t.Errorf("receipts[0].TotalMoney = %v, want 1000", receipts[0].TotalMoney)
	}
	if receipts[2].TotalMoney != 3000 {
		t.Errorf("receipts[2].TotalMoney = %v, want 3000", receipts[2].TotalMoney)
	}
}

func TestSortReceipts_ByDate(t *testing.T) {
	now := time.Now()
	receipts := []loyverse.Receipt{
		{ReceiptNumber: "newest", CreatedAt: now},
		{ReceiptNumber: "oldest", CreatedAt: now.Add(-48 * time.Hour)},
		{ReceiptNumber: "middle", CreatedAt: now.Add(-24 * time.Hour)},
	}

	loyverse.SortReceipts(receipts, loyverse.SortByDate, loyverse.SortAsc)

	if receipts[0].ReceiptNumber != "oldest" {
		t.Errorf("receipts[0].ReceiptNumber = %q, want oldest", receipts[0].ReceiptNumber)
	}
	if receipts[2].ReceiptNumber != "newest" {
		t.Errorf("receipts[2].ReceiptNumber = %q, want newest", receipts[2].ReceiptNumber)
	}
}

func TestSortCategories_Asc(t *testing.T) {
	cats := []loyverse.Category{
		{Name: "Snacks"},
		{Name: "Bebidas"},
		{Name: "Limpieza"},
	}

	loyverse.SortCategories(cats, loyverse.SortAsc)

	if cats[0].Name != "Bebidas" {
		t.Errorf("cats[0].Name = %q, want Bebidas", cats[0].Name)
	}
}
