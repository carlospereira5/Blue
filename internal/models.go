package internal

type Transaction struct {
	ID            int
	TotalProducts []LineItem
	TotalMoney    int
}

type LineItem struct {
	Product  string
	Quantity int
}

type AccountingBook struct {
	Transactions []Transaction
}

type Inventory struct {
	Products map[string]int
}
