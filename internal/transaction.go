package internal

func (t *Transaction) MoneyIn() int {
	return t.TotalMoney
}

func (t *Transaction) ProductsOUt() []LineItem {
	return t.TotalProducts
}

func (ab *AccountingBook) AddTransaction(t Transaction) {
	ab.Transactions = append(ab.Transactions, t)
}
