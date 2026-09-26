package payments

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/money"
)

type HoldInput struct {
	OrderID    uuid.UUID
	PaymentID  uuid.UUID
	CustomerID uuid.UUID
	Method     Method
	Total      money.Money
}

type ReleaseInput struct {
	OrderID      uuid.UUID
	PaymentID    uuid.UUID
	CustomerID   uuid.UUID
	MerchantID   *uuid.UUID
	RiderID      uuid.UUID
	Method       Method
	ItemTotal    money.Money
	DeliveryFee  money.Money
	Commission   money.Money
	RiderEarning money.Money
}

type RefundInput struct {
	OrderID    uuid.UUID
	PaymentID  uuid.UUID
	CustomerID uuid.UUID
	Method     Method
	Total      money.Money
}

type AdjustInput struct {
	CustomerID uuid.UUID
	Amount     money.Money
	RequestID  string
}

type SettleInput struct {
	RiderID   uuid.UUID
	Amount    money.Money
	RequestID string
}

type Leg struct {
	Kind      AccountKind
	AccountID uuid.UUID
	Direction Direction
	Amount    money.Money
	Key       string
}

type Journal struct {
	ID        uuid.UUID
	OrderID   *uuid.UUID
	PaymentID *uuid.UUID
	Type      EntryType
	Legs      []Leg
}

func HoldJournal(in HoldInput) (Journal, error) {
	build, ok := holdByMethod[in.Method]
	if !ok {
		return Journal{}, apperror.Invalid("payment_method is invalid")
	}
	return finish(build(in))
}

func ReleaseJournal(in ReleaseInput) (Journal, error) {
	build, ok := releaseByMethod[in.Method]
	if !ok {
		return Journal{}, apperror.Invalid("payment_method is invalid")
	}
	journal, err := build(in)
	if err != nil {
		return Journal{}, err
	}
	return finish(journal)
}

func RefundJournal(in RefundInput) (Journal, error) {
	build, ok := refundByMethod[in.Method]
	if !ok {
		return Journal{}, apperror.Invalid("payment_method is invalid")
	}
	return finish(build(in))
}

func AdjustJournal(in AdjustInput) (Journal, error) {
	if in.Amount.IsZero() || in.Amount.IsNegative() || in.RequestID == "" {
		return Journal{}, apperror.Invalid("amount is invalid")
	}
	root := "adjust:" + in.RequestID
	return finish(Journal{
		ID:   uuid.New(),
		Type: EntryWalletAdjustment,
		Legs: []Leg{
			leg(KindAdjustment, PlatformAccountID, Debit, in.Amount, root),
			leg(KindCustomerWallet, in.CustomerID, Credit, in.Amount, root),
		},
	})
}

func SettleJournal(in SettleInput) (Journal, error) {
	if in.Amount.IsZero() || in.Amount.IsNegative() || in.RequestID == "" {
		return Journal{}, apperror.Invalid("amount is invalid")
	}
	root := "settle:" + in.RequestID
	return finish(Journal{
		ID:   uuid.New(),
		Type: EntryCODSettled,
		Legs: []Leg{
			leg(KindRiderCashOwed, in.RiderID, Credit, in.Amount, root),
			leg(KindSettlementCash, PlatformAccountID, Debit, in.Amount, root),
		},
	})
}

func AssertBalanced(journal Journal) error {
	if len(journal.Legs) == 0 {
		return nil
	}
	debit, credit := totals(journal)
	if debit != credit {
		return fmt.Errorf("payments: unbalanced journal")
	}
	return nil
}

func totals(journal Journal) (int64, int64) {
	var debit, credit int64
	for _, item := range journal.Legs {
		debit, credit = addLeg(debit, credit, item)
	}
	return debit, credit
}

func addLeg(debit, credit int64, item Leg) (int64, int64) {
	if item.Direction == Debit {
		return debit + item.Amount.Paisa(), credit
	}
	return debit, credit + item.Amount.Paisa()
}

func finish(journal Journal) (Journal, error) {
	journal.Legs = compact(journal.Legs)
	if err := AssertBalanced(journal); err != nil {
		return Journal{}, err
	}
	return journal, nil
}

func compact(legs []Leg) []Leg {
	out := make([]Leg, 0, len(legs))
	for _, item := range legs {
		if item.Amount.IsZero() {
			continue
		}
		out = append(out, item)
	}
	return out
}

var holdByMethod = map[Method]func(HoldInput) Journal{
	MethodWallet:    holdWallet,
	MethodJazzCash:  holdGateway,
	MethodEasyPaisa: holdGateway,
	MethodCOD:       holdEmpty,
}

var releaseByMethod = map[Method]func(ReleaseInput) (Journal, error){
	MethodWallet:    digitalRelease,
	MethodJazzCash:  digitalRelease,
	MethodEasyPaisa: digitalRelease,
	MethodCOD:       codRelease,
}

var refundByMethod = map[Method]func(RefundInput) Journal{
	MethodWallet:    refundWallet,
	MethodJazzCash:  refundGateway,
	MethodEasyPaisa: refundGateway,
	MethodCOD:       refundEmpty,
}

func holdEmpty(HoldInput) Journal { return Journal{} }

func holdWallet(in HoldInput) Journal {
	root := in.OrderID.String() + ":" + string(EntryEscrowHold)
	return orderJournal(in.OrderID, in.PaymentID, EntryEscrowHold, []Leg{
		leg(KindCustomerWallet, in.CustomerID, Debit, in.Total, root),
		leg(KindEscrow, in.OrderID, Credit, in.Total, root),
	})
}

func holdGateway(in HoldInput) Journal {
	root := in.OrderID.String() + ":" + string(EntryEscrowHold)
	return orderJournal(in.OrderID, in.PaymentID, EntryEscrowHold, []Leg{
		leg(KindGatewayClearing, PlatformAccountID, Debit, in.Total, root),
		leg(KindEscrow, in.OrderID, Credit, in.Total, root),
	})
}

func digitalRelease(in ReleaseInput) (Journal, error) {
	total, err := in.ItemTotal.Add(in.DeliveryFee)
	if err != nil {
		return Journal{}, err
	}
	share, err := payable(in)
	if err != nil {
		return Journal{}, err
	}
	platform, err := subtractAll(total, share, in.RiderEarning)
	if err != nil {
		return Journal{}, err
	}
	root := in.OrderID.String() + ":" + string(EntryEscrowRelease)
	return orderJournal(in.OrderID, in.PaymentID, EntryEscrowRelease, []Leg{
		leg(KindEscrow, in.OrderID, Debit, total, root),
		merchantCredit(in, share, root),
		leg(KindRiderEarnings, in.RiderID, Credit, in.RiderEarning, root),
		leg(KindPlatformRevenue, PlatformAccountID, Credit, platform, root),
	}), nil
}

func codRelease(in ReleaseInput) (Journal, error) {
	remit, err := remittance(in)
	if err != nil {
		return Journal{}, err
	}
	share, err := payable(in)
	if err != nil {
		return Journal{}, err
	}
	platform, err := subtractAll(remit, share)
	if err != nil {
		return Journal{}, err
	}
	root := in.OrderID.String() + ":" + string(EntryCODRecognized)
	return orderJournal(in.OrderID, in.PaymentID, EntryCODRecognized, []Leg{
		leg(KindRiderCashOwed, in.RiderID, Debit, remit, root),
		merchantCredit(in, share, root),
		leg(KindPlatformRevenue, PlatformAccountID, Credit, platform, root),
	}), nil
}

func refundEmpty(RefundInput) Journal { return Journal{} }

func refundWallet(in RefundInput) Journal {
	root := in.OrderID.String() + ":" + string(EntryRefund)
	return orderJournal(in.OrderID, in.PaymentID, EntryRefund, []Leg{
		leg(KindEscrow, in.OrderID, Debit, in.Total, root),
		leg(KindCustomerWallet, in.CustomerID, Credit, in.Total, root),
	})
}

func refundGateway(in RefundInput) Journal {
	root := in.OrderID.String() + ":" + string(EntryRefund)
	return orderJournal(in.OrderID, in.PaymentID, EntryRefund, []Leg{
		leg(KindEscrow, in.OrderID, Debit, in.Total, root),
		leg(KindGatewayClearing, PlatformAccountID, Credit, in.Total, root),
	})
}

func orderJournal(orderID, paymentID uuid.UUID, kind EntryType, legs []Leg) Journal {
	return Journal{ID: uuid.New(), OrderID: &orderID, PaymentID: &paymentID, Type: kind, Legs: legs}
}

func leg(kind AccountKind, account uuid.UUID, direction Direction, amount money.Money, root string) Leg {
	return Leg{
		Kind: kind, AccountID: account, Direction: direction, Amount: amount,
		Key: root + ":" + string(kind) + ":" + string(direction),
	}
}

func merchantCredit(in ReleaseInput, share money.Money, root string) Leg {
	if in.MerchantID == nil {
		return Leg{}
	}
	return leg(KindMerchantPayable, *in.MerchantID, Credit, share, root)
}

func payable(in ReleaseInput) (money.Money, error) {
	if in.MerchantID == nil {
		return money.FromPaisa(0), nil
	}
	share, err := in.ItemTotal.Sub(in.Commission)
	if err != nil {
		return money.Money{}, err
	}
	return nonNegative(share), nil
}

func remittance(in ReleaseInput) (money.Money, error) {
	total, err := in.ItemTotal.Add(in.DeliveryFee)
	if err != nil {
		return money.Money{}, err
	}
	return total.Sub(in.RiderEarning)
}

func subtractAll(total money.Money, parts ...money.Money) (money.Money, error) {
	left := total
	for _, part := range parts {
		next, err := left.Sub(part)
		if err != nil {
			return money.Money{}, err
		}
		left = next
	}
	if left.IsNegative() {
		return money.Money{}, fmt.Errorf("payments: unbalanced journal")
	}
	return left, nil
}

func nonNegative(amount money.Money) money.Money {
	if amount.IsNegative() {
		return money.FromPaisa(0)
	}
	return amount
}

func GatewayStubRef(method Method, orderID uuid.UUID) (string, bool) {
	if method != MethodJazzCash && method != MethodEasyPaisa {
		return "", false
	}
	return "stub-" + orderID.String(), true
}
