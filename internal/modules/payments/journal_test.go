package payments

import (
	"testing"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/money"
)

func TestJournalsBalance(t *testing.T) {
	orderID := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1")
	paymentID := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa2")
	customerID := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa3")
	merchantID := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa4")
	riderID := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa5")
	item := money.FromPaisa(90000)
	fee := money.FromPaisa(8174)
	commission := money.FromPaisa(16200)
	rider := money.FromPaisa(6539)
	total, err := item.Add(fee)
	if err != nil {
		t.Fatal(err)
	}
	release := ReleaseInput{
		OrderID: orderID, PaymentID: paymentID, CustomerID: customerID, MerchantID: &merchantID, RiderID: riderID,
		ItemTotal: item, DeliveryFee: fee, Commission: commission, RiderEarning: rider,
	}
	cases := []struct {
		name        string
		journal     Journal
		debit       AccountKind
		credit      AccountKind
		paisa       int64
		creditPaisa int64
		absent      AccountKind
	}{
		{name: "wallet hold", journal: mustHold(t, MethodWallet, orderID, paymentID, customerID, total), debit: KindCustomerWallet, credit: KindEscrow, paisa: total.Paisa()},
		{name: "gateway hold", journal: mustHold(t, MethodJazzCash, orderID, paymentID, customerID, total), debit: KindGatewayClearing, credit: KindEscrow, paisa: total.Paisa()},
		{name: "cod hold", journal: mustHold(t, MethodCOD, orderID, paymentID, customerID, total)},
		{name: "digital release", journal: mustRelease(t, MethodWallet, release), debit: KindEscrow, credit: KindRiderEarnings, paisa: total.Paisa(), creditPaisa: rider.Paisa()},
		{name: "cod delivery", journal: mustRelease(t, MethodCOD, release), debit: KindRiderCashOwed, credit: KindMerchantPayable, paisa: 91635, creditPaisa: 73800, absent: KindRiderEarnings},
		{name: "wallet refund", journal: mustRefund(t, MethodWallet, orderID, paymentID, customerID, total), debit: KindEscrow, credit: KindCustomerWallet, paisa: total.Paisa()},
		{name: "cod refund", journal: mustRefund(t, MethodCOD, orderID, paymentID, customerID, total)},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertBalanced(t, tt.journal)
			if tt.paisa == 0 {
				return
			}
			if got := amountOf(tt.journal, tt.debit, Debit); got != tt.paisa {
				t.Fatalf("debit %s = %d, want %d", tt.debit, got, tt.paisa)
			}
			if tt.creditPaisa != 0 && amountOf(tt.journal, tt.credit, Credit) != tt.creditPaisa {
				t.Fatalf("credit %s = %d, want %d", tt.credit, amountOf(tt.journal, tt.credit, Credit), tt.creditPaisa)
			}
			if tt.absent != "" && amountOf(tt.journal, tt.absent, Credit) != 0 {
				t.Fatalf("did not expect %s", tt.absent)
			}
		})
	}
}

func TestAdjustAndSettleBalance(t *testing.T) {
	amount := money.FromPaisa(5000)
	adjust, err := AdjustJournal(AdjustInput{CustomerID: uuid.New(), Amount: amount, RequestID: "credit-1"})
	if err != nil {
		t.Fatal(err)
	}
	settle, err := SettleJournal(SettleInput{RiderID: uuid.New(), Amount: amount, RequestID: "settle-1"})
	if err != nil {
		t.Fatal(err)
	}
	assertBalanced(t, adjust)
	assertBalanced(t, settle)
	if adjust.Type != EntryWalletAdjustment || settle.Type != EntryCODSettled {
		t.Fatalf("types %s %s", adjust.Type, settle.Type)
	}
}

func mustHold(t *testing.T, method Method, orderID, paymentID, customerID uuid.UUID, total money.Money) Journal {
	t.Helper()
	journal, err := HoldJournal(HoldInput{OrderID: orderID, PaymentID: paymentID, CustomerID: customerID, Method: method, Total: total})
	if err != nil {
		t.Fatal(err)
	}
	return journal
}

func mustRelease(t *testing.T, method Method, in ReleaseInput) Journal {
	t.Helper()
	in.Method = method
	journal, err := ReleaseJournal(in)
	if err != nil {
		t.Fatal(err)
	}
	return journal
}

func mustRefund(t *testing.T, method Method, orderID, paymentID, customerID uuid.UUID, total money.Money) Journal {
	t.Helper()
	journal, err := RefundJournal(RefundInput{OrderID: orderID, PaymentID: paymentID, CustomerID: customerID, Method: method, Total: total})
	if err != nil {
		t.Fatal(err)
	}
	return journal
}

func assertBalanced(t *testing.T, journal Journal) {
	t.Helper()
	debit, credit := totals(journal)
	if debit != credit {
		t.Fatalf("debit %d credit %d", debit, credit)
	}
}

func amountOf(journal Journal, kind AccountKind, direction Direction) int64 {
	for _, item := range journal.Legs {
		if item.Kind == kind && item.Direction == direction {
			return item.Amount.Paisa()
		}
	}
	return 0
}
