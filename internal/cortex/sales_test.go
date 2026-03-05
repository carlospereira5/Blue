package cortex_test

import (
	"testing"
	"time"

	"aria/internal/cortex"
	"aria/internal/loyverse"
)

func TestCalculateSalesMetrics(t *testing.T) {
	now := time.Now()
	cancelled := now

	tests := []struct {
		name     string
		receipts []loyverse.Receipt
		want     func(t *testing.T, m cortex.SalesMetrics)
	}{
		{
			name:     "empty receipts",
			receipts: nil,
			want: func(t *testing.T, m cortex.SalesMetrics) {
				t.Helper()
				assertFloat(t, "GrossSales", 0, m.GrossSales)
				assertFloat(t, "NetSales", 0, m.NetSales)
				assertFloat(t, "TotalRefund", 0, m.TotalRefund)
				assertInt(t, "SalesCount", 0, m.SalesCount)
				assertInt(t, "RefundCount", 0, m.RefundCount)
				assertInt(t, "ByPaymentMethod len", 0, len(m.ByPaymentMethod))
			},
		},
		{
			name: "single sale",
			receipts: []loyverse.Receipt{
				{
					ReceiptType: "SALE",
					TotalMoney:  10000,
					TotalTax:    1900,
					Tip:         500,
					Payments: []loyverse.Payment{
						{Name: "Efectivo", MoneyAmount: 10000},
					},
				},
			},
			want: func(t *testing.T, m cortex.SalesMetrics) {
				t.Helper()
				assertFloat(t, "GrossSales", 10000, m.GrossSales)
				assertFloat(t, "NetSales", 10000, m.NetSales)
				assertFloat(t, "TotalTax", 1900, m.TotalTax)
				assertFloat(t, "TotalTip", 500, m.TotalTip)
				assertInt(t, "SalesCount", 1, m.SalesCount)
				assertInt(t, "RefundCount", 0, m.RefundCount)
				assertFloat(t, "Efectivo.Sales", 10000, m.ByPaymentMethod["Efectivo"].Sales)
				assertInt(t, "Efectivo.Count", 1, m.ByPaymentMethod["Efectivo"].Count)
			},
		},
		{
			name: "sale with discount",
			receipts: []loyverse.Receipt{
				{
					ReceiptType:   "SALE",
					TotalMoney:    8000,
					TotalDiscount: 2000,
					Payments: []loyverse.Payment{
						{Name: "Tarjeta", MoneyAmount: 8000},
					},
				},
			},
			want: func(t *testing.T, m cortex.SalesMetrics) {
				t.Helper()
				assertFloat(t, "GrossSales", 8000, m.GrossSales)
				assertFloat(t, "TotalDiscount", 2000, m.TotalDiscount)
				assertFloat(t, "NetSales", 6000, m.NetSales) // 8000 - 2000 - 0
			},
		},
		{
			name: "sale and refund",
			receipts: []loyverse.Receipt{
				{
					ReceiptType: "SALE",
					TotalMoney:  15000,
					Payments: []loyverse.Payment{
						{Name: "Efectivo", MoneyAmount: 15000},
					},
				},
				{
					ReceiptType: "REFUND",
					TotalMoney:  -5000,
					Payments: []loyverse.Payment{
						{Name: "Efectivo", MoneyAmount: 5000},
					},
				},
			},
			want: func(t *testing.T, m cortex.SalesMetrics) {
				t.Helper()
				assertFloat(t, "GrossSales", 15000, m.GrossSales)
				assertFloat(t, "TotalRefund", 5000, m.TotalRefund)
				assertFloat(t, "NetSales", 10000, m.NetSales) // 15000 - 0 - 5000
				assertInt(t, "SalesCount", 1, m.SalesCount)
				assertInt(t, "RefundCount", 1, m.RefundCount)
				assertFloat(t, "Efectivo.Sales", 15000, m.ByPaymentMethod["Efectivo"].Sales)
				assertFloat(t, "Efectivo.Refunds", 5000, m.ByPaymentMethod["Efectivo"].Refunds)
			},
		},
		{
			name: "cancelled receipt is skipped",
			receipts: []loyverse.Receipt{
				{
					ReceiptType: "SALE",
					TotalMoney:  10000,
					CancelledAt: &cancelled,
					Payments: []loyverse.Payment{
						{Name: "Efectivo", MoneyAmount: 10000},
					},
				},
			},
			want: func(t *testing.T, m cortex.SalesMetrics) {
				t.Helper()
				assertFloat(t, "GrossSales", 0, m.GrossSales)
				assertInt(t, "SalesCount", 0, m.SalesCount)
				assertInt(t, "ByPaymentMethod len", 0, len(m.ByPaymentMethod))
			},
		},
		{
			name: "multiple payment methods",
			receipts: []loyverse.Receipt{
				{
					ReceiptType: "SALE",
					TotalMoney:  20000,
					Payments: []loyverse.Payment{
						{Name: "Efectivo", MoneyAmount: 12000},
						{Name: "Tarjeta", MoneyAmount: 8000},
					},
				},
			},
			want: func(t *testing.T, m cortex.SalesMetrics) {
				t.Helper()
				assertFloat(t, "GrossSales", 20000, m.GrossSales)
				assertFloat(t, "Efectivo.Sales", 12000, m.ByPaymentMethod["Efectivo"].Sales)
				assertFloat(t, "Tarjeta.Sales", 8000, m.ByPaymentMethod["Tarjeta"].Sales)
			},
		},
		{
			name: "payment name falls back to PaymentTypeID",
			receipts: []loyverse.Receipt{
				{
					ReceiptType: "SALE",
					TotalMoney:  5000,
					Payments: []loyverse.Payment{
						{PaymentTypeID: "pt-abc-123", MoneyAmount: 5000},
					},
				},
			},
			want: func(t *testing.T, m cortex.SalesMetrics) {
				t.Helper()
				assertFloat(t, "fallback.Sales", 5000, m.ByPaymentMethod["pt-abc-123"].Sales)
			},
		},
		{
			name: "negative total_money on refund uses absolute value",
			receipts: []loyverse.Receipt{
				{
					ReceiptType: "REFUND",
					TotalMoney:  -3000,
					Payments: []loyverse.Payment{
						{Name: "Efectivo", MoneyAmount: 3000},
					},
				},
			},
			want: func(t *testing.T, m cortex.SalesMetrics) {
				t.Helper()
				assertFloat(t, "TotalRefund", 3000, m.TotalRefund)
				assertInt(t, "RefundCount", 1, m.RefundCount)
			},
		},
		{
			name: "multiple sales accumulate correctly",
			receipts: []loyverse.Receipt{
				{
					ReceiptType:   "SALE",
					TotalMoney:    10000,
					TotalDiscount: 1000,
					TotalTax:      1900,
					Tip:           200,
					Payments:      []loyverse.Payment{{Name: "Efectivo", MoneyAmount: 10000}},
				},
				{
					ReceiptType:   "SALE",
					TotalMoney:    5000,
					TotalDiscount: 500,
					TotalTax:      950,
					Tip:           100,
					Payments:      []loyverse.Payment{{Name: "Efectivo", MoneyAmount: 5000}},
				},
			},
			want: func(t *testing.T, m cortex.SalesMetrics) {
				t.Helper()
				assertFloat(t, "GrossSales", 15000, m.GrossSales)
				assertFloat(t, "TotalDiscount", 1500, m.TotalDiscount)
				assertFloat(t, "TotalTax", 2850, m.TotalTax)
				assertFloat(t, "TotalTip", 300, m.TotalTip)
				assertFloat(t, "NetSales", 13500, m.NetSales) // 15000 - 1500 - 0
				assertInt(t, "SalesCount", 2, m.SalesCount)
				assertFloat(t, "Efectivo.Sales", 15000, m.ByPaymentMethod["Efectivo"].Sales)
				assertInt(t, "Efectivo.Count", 2, m.ByPaymentMethod["Efectivo"].Count)
			},
		},
		{
			name: "full scenario: sales + refunds + discounts + cancelled",
			receipts: []loyverse.Receipt{
				{
					ReceiptType:   "SALE",
					TotalMoney:    20000,
					TotalDiscount: 2000,
					TotalTax:      3800,
					Payments: []loyverse.Payment{
						{Name: "Efectivo", MoneyAmount: 12000},
						{Name: "Tarjeta", MoneyAmount: 8000},
					},
				},
				{
					ReceiptType: "SALE",
					TotalMoney:  5000,
					Payments: []loyverse.Payment{
						{Name: "Efectivo", MoneyAmount: 5000},
					},
				},
				{
					ReceiptType: "REFUND",
					TotalMoney:  -3000,
					Payments: []loyverse.Payment{
						{Name: "Efectivo", MoneyAmount: 3000},
					},
				},
				{
					ReceiptType: "SALE",
					TotalMoney:  10000,
					CancelledAt: &cancelled, // skipped
					Payments: []loyverse.Payment{
						{Name: "Efectivo", MoneyAmount: 10000},
					},
				},
			},
			want: func(t *testing.T, m cortex.SalesMetrics) {
				t.Helper()
				assertFloat(t, "GrossSales", 25000, m.GrossSales)       // 20000 + 5000
				assertFloat(t, "TotalDiscount", 2000, m.TotalDiscount)  // only first sale
				assertFloat(t, "TotalRefund", 3000, m.TotalRefund)      // one refund
				assertFloat(t, "NetSales", 20000, m.NetSales)           // 25000 - 2000 - 3000
				assertInt(t, "SalesCount", 2, m.SalesCount)             // cancelled skipped
				assertInt(t, "RefundCount", 1, m.RefundCount)
				assertFloat(t, "Efectivo.Sales", 17000, m.ByPaymentMethod["Efectivo"].Sales)   // 12000 + 5000
				assertFloat(t, "Efectivo.Refunds", 3000, m.ByPaymentMethod["Efectivo"].Refunds)
				assertFloat(t, "Tarjeta.Sales", 8000, m.ByPaymentMethod["Tarjeta"].Sales)
				assertFloat(t, "Tarjeta.Refunds", 0, m.ByPaymentMethod["Tarjeta"].Refunds)
			},
		},
	}

	_ = now // suppress unused

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cortex.CalculateSalesMetrics(tt.receipts)
			tt.want(t, got)
		})
	}
}

func assertFloat(t *testing.T, label string, want, got float64) {
	t.Helper()
	if want != got {
		t.Errorf("%s: want %.2f, got %.2f", label, want, got)
	}
}

func assertInt(t *testing.T, label string, want, got int) {
	t.Helper()
	if want != got {
		t.Errorf("%s: want %d, got %d", label, want, got)
	}
}
