package payments

import "github.com/google/uuid"

type Method string

const (
	MethodJazzCash  Method = "jazzcash"
	MethodEasyPaisa Method = "easypaisa"
	MethodCOD       Method = "cod"
	MethodWallet    Method = "wallet"
)

func Methods() []Method {
	return []Method{MethodJazzCash, MethodEasyPaisa, MethodCOD, MethodWallet}
}

type Status string

const (
	StatusHeld     Status = "held"
	StatusReleased Status = "released"
	StatusRefunded Status = "refunded"
	StatusFailed   Status = "failed"
)

func Statuses() []Status {
	return []Status{StatusHeld, StatusReleased, StatusRefunded, StatusFailed}
}

type AccountKind string

const (
	KindCustomerWallet  AccountKind = "customer_wallet"
	KindRiderEarnings   AccountKind = "rider_earnings"
	KindRiderCashOwed   AccountKind = "rider_cash_owed"
	KindMerchantPayable AccountKind = "merchant_payable"
	KindPlatformRevenue AccountKind = "platform_revenue"
	KindEscrow          AccountKind = "escrow"
	KindGatewayClearing AccountKind = "gateway_clearing"
	KindSettlementCash  AccountKind = "settlement_cash"
	KindAdjustment      AccountKind = "adjustment"
)

func AccountKinds() []AccountKind {
	return []AccountKind{
		KindCustomerWallet,
		KindRiderEarnings,
		KindRiderCashOwed,
		KindMerchantPayable,
		KindPlatformRevenue,
		KindEscrow,
		KindGatewayClearing,
		KindSettlementCash,
		KindAdjustment,
	}
}

type Direction string

const (
	Debit  Direction = "debit"
	Credit Direction = "credit"
)

func Directions() []Direction {
	return []Direction{Debit, Credit}
}

type EntryType string

const (
	EntryEscrowHold       EntryType = "escrow_hold"
	EntryEscrowRelease    EntryType = "escrow_release"
	EntryRefund           EntryType = "refund"
	EntryCODRecognized    EntryType = "cod_recognized"
	EntryCODSettled       EntryType = "cod_settled"
	EntryWalletAdjustment EntryType = "wallet_adjustment"
)

func EntryTypes() []EntryType {
	return []EntryType{
		EntryEscrowHold,
		EntryEscrowRelease,
		EntryRefund,
		EntryCODRecognized,
		EntryCODSettled,
		EntryWalletAdjustment,
	}
}

// PlatformAccountID is the ledger account for platform revenue, gateway clearing, and settlement cash.
var PlatformAccountID = uuid.MustParse("00000000-0000-4000-8000-000000000001")

// RiderDeliveryShareBPS is the rider share of the delivery fee. 8000 means 80 percent.
// The order row stores the resulting rider_earning. Item commission is separate.
const RiderDeliveryShareBPS int64 = 8000
