package cortex_test

import (
	"testing"
	"time"

	"aria/internal/cortex"
	"aria/internal/loyverse"
)

func TestCalculateCashFlow(t *testing.T) {
	cancelled := time.Now()

	makeReceipt := func(typ string, total, discount float64) loyverse.Receipt {
		return loyverse.Receipt{
			ReceiptType:   typ,
			TotalMoney:    total,
			TotalDiscount: discount,
		}
	}

	makeShift := func(movements ...loyverse.CashMovement) loyverse.Shift {
		return loyverse.Shift{CashMovements: movements}
	}

	tests := []struct {
		name     string
		receipts []loyverse.Receipt
		shifts   []loyverse.Shift
		want     cortex.CashFlowResult
	}{
		{
			name: "only sales, no movements",
			receipts: []loyverse.Receipt{
				makeReceipt("SALE", 1000, 0),
				makeReceipt("SALE", 500, 50),
			},
			want: cortex.CashFlowResult{
				NetSales:    1450, // 1000 + (500-50)
				NetCashFlow: 1450,
			},
		},
		{
			name: "sales with refunds reduce net",
			receipts: []loyverse.Receipt{
				makeReceipt("SALE", 1000, 0),
				makeReceipt("REFUND", -200, 0), // Loyverse envía negativos en refunds
			},
			want: cortex.CashFlowResult{
				NetSales:    800, // 1000 - 200
				NetCashFlow: 800,
			},
		},
		{
			name: "payouts reduce net cash flow",
			receipts: []loyverse.Receipt{
				makeReceipt("SALE", 1000, 0),
			},
			shifts: []loyverse.Shift{
				makeShift(
					loyverse.CashMovement{Type: "PAY_OUT", MoneyAmount: 300},
					loyverse.CashMovement{Type: "PAY_OUT", MoneyAmount: 200},
				),
			},
			want: cortex.CashFlowResult{
				NetSales:    1000,
				TotalPayOut: 500,
				NetCashFlow: 500, // 1000 - 500
			},
		},
		{
			name: "payins add to net cash flow",
			receipts: []loyverse.Receipt{
				makeReceipt("SALE", 1000, 0),
			},
			shifts: []loyverse.Shift{
				makeShift(loyverse.CashMovement{Type: "PAY_IN", MoneyAmount: 150}),
			},
			want: cortex.CashFlowResult{
				NetSales:    1000,
				TotalPayIn:  150,
				NetCashFlow: 1150,
			},
		},
		{
			name: "cancelled receipts excluded",
			receipts: []loyverse.Receipt{
				makeReceipt("SALE", 500, 0),
				{
					ReceiptType: "SALE",
					TotalMoney:  9999,
					CancelledAt: &cancelled,
				},
			},
			want: cortex.CashFlowResult{
				NetSales:    500, // el cancelado (9999) no cuenta
				NetCashFlow: 500,
			},
		},
		{
			name:     "empty inputs",
			receipts: nil,
			shifts:   nil,
			want:     cortex.CashFlowResult{},
		},
		{
			name: "full scenario: sales + refund + payout + payin",
			receipts: []loyverse.Receipt{
				makeReceipt("SALE", 2000, 100),  // neto 1900
				makeReceipt("SALE", 500, 0),     // neto 500
				makeReceipt("REFUND", -300, 0),  // resta 300
			},
			shifts: []loyverse.Shift{
				makeShift(
					loyverse.CashMovement{Type: "PAY_OUT", MoneyAmount: 400},
					loyverse.CashMovement{Type: "PAY_IN", MoneyAmount: 50},
				),
			},
			want: cortex.CashFlowResult{
				NetSales:    2100, // 1900 + 500 - 300
				TotalPayOut: 400,
				TotalPayIn:  50,
				NetCashFlow: 1750, // 2100 + 50 - 400
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := cortex.CalculateCashFlow(tc.receipts, tc.shifts)

			assertFloat(t, "NetSales", tc.want.NetSales, got.NetSales)
			assertFloat(t, "TotalPayOut", tc.want.TotalPayOut, got.TotalPayOut)
			assertFloat(t, "TotalPayIn", tc.want.TotalPayIn, got.TotalPayIn)
			assertFloat(t, "NetCashFlow", tc.want.NetCashFlow, got.NetCashFlow)
		})
	}
}
