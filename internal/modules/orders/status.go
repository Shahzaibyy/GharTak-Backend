package orders

import "errors"

type Type string

const (
	TypeFood    Type = "food"
	TypeMart    Type = "mart"
	TypeCourier Type = "courier"
	TypeErrand  Type = "errand"
)

func Types() []Type {
	return []Type{TypeFood, TypeMart, TypeCourier, TypeErrand}
}

type Status string

const (
	StatusPlaced           Status = "placed"
	StatusMerchantAccepted Status = "merchant_accepted"
	StatusPreparing        Status = "preparing"
	StatusReadyForPickup   Status = "ready_for_pickup"
	StatusRiderOffered     Status = "rider_offered"
	StatusAccepted         Status = "accepted"
	StatusPickedUp         Status = "picked_up"
	StatusOnTheWay         Status = "on_the_way"
	StatusDelivered        Status = "delivered"
	StatusCancelled        Status = "cancelled"
	StatusRejected         Status = "rejected"
)

func Statuses() []Status {
	return []Status{
		StatusPlaced,
		StatusMerchantAccepted,
		StatusPreparing,
		StatusReadyForPickup,
		StatusRiderOffered,
		StatusAccepted,
		StatusPickedUp,
		StatusOnTheWay,
		StatusDelivered,
		StatusCancelled,
		StatusRejected,
	}
}

type EffortTier string

const (
	EffortLow    EffortTier = "low"
	EffortMedium EffortTier = "medium"
	EffortHigh   EffortTier = "high"
)

func EffortTiers() []EffortTier {
	return []EffortTier{EffortLow, EffortMedium, EffortHigh}
}

var (
	ErrUnknownType       = errors.New("unknown order type")
	ErrInvalidTransition = errors.New("invalid order transition")
	ErrUnknownEffort     = errors.New("unknown effort tier")
)

var terminal = map[Status]struct{}{
	StatusDelivered: {},
	StatusCancelled: {},
	StatusRejected:  {},
}

var merchantTypes = map[Type]struct{}{
	TypeFood: {},
	TypeMart: {},
}

var effortBPS = map[EffortTier]int64{
	EffortLow:    10000,
	EffortMedium: 13000,
	EffortHigh:   16000,
}

var merchantFlow = map[Status]map[Status]struct{}{
	StatusPlaced:           allow(StatusMerchantAccepted, StatusRejected, StatusCancelled),
	StatusMerchantAccepted: allow(StatusPreparing, StatusCancelled),
	StatusPreparing:        allow(StatusReadyForPickup, StatusCancelled),
	StatusReadyForPickup:   allow(StatusRiderOffered, StatusCancelled),
	StatusRiderOffered:     allow(StatusAccepted, StatusCancelled),
	StatusAccepted:         allow(StatusPickedUp, StatusCancelled),
	StatusPickedUp:         allow(StatusOnTheWay, StatusCancelled),
	StatusOnTheWay:         allow(StatusDelivered, StatusCancelled),
}

var directFlow = map[Status]map[Status]struct{}{
	StatusPlaced:       allow(StatusRiderOffered, StatusCancelled),
	StatusRiderOffered: allow(StatusAccepted, StatusCancelled),
	StatusAccepted:     allow(StatusPickedUp, StatusCancelled),
	StatusPickedUp:     allow(StatusOnTheWay, StatusCancelled),
	StatusOnTheWay:     allow(StatusDelivered, StatusCancelled),
}

var transitions = map[Type]map[Status]map[Status]struct{}{
	TypeFood:    merchantFlow,
	TypeMart:    merchantFlow,
	TypeCourier: directFlow,
	TypeErrand:  directFlow,
}

func RequiresMerchant(kind Type) bool {
	_, ok := merchantTypes[kind]
	return ok
}

func IsTerminal(status Status) bool {
	_, ok := terminal[status]
	return ok
}

func CanTransition(kind Type, from, to Status) bool {
	next, ok := transitions[kind][from]
	if !ok {
		return false
	}
	_, ok = next[to]
	return ok
}

func Transition(kind Type, from, to Status) error {
	if _, ok := transitions[kind]; !ok {
		return ErrUnknownType
	}
	if !CanTransition(kind, from, to) {
		return ErrInvalidTransition
	}
	return nil
}

// EffortBPS scales an errand or courier fee. 10000 is 1.00x.
func EffortBPS(tier EffortTier) (int64, error) {
	bps, ok := effortBPS[tier]
	if !ok {
		return 0, ErrUnknownEffort
	}
	return bps, nil
}

func allow(targets ...Status) map[Status]struct{} {
	out := make(map[Status]struct{}, len(targets))
	for _, target := range targets {
		out[target] = struct{}{}
	}
	return out
}
