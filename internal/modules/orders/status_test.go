package orders_test

import (
	"errors"
	"testing"

	"github.com/yourusername/ghartak-backend/internal/modules/orders"
)

func TestTransition(t *testing.T) {
	tests := []struct {
		name    string
		kind    orders.Type
		from    orders.Status
		to      orders.Status
		wantErr error
	}{
		{name: "food accept", kind: orders.TypeFood, from: orders.StatusPlaced, to: orders.StatusMerchantAccepted},
		{name: "food reject", kind: orders.TypeFood, from: orders.StatusPlaced, to: orders.StatusRejected},
		{name: "food skip merchant", kind: orders.TypeFood, from: orders.StatusPlaced, to: orders.StatusRiderOffered, wantErr: orders.ErrInvalidTransition},
		{name: "food deliver early", kind: orders.TypeFood, from: orders.StatusPlaced, to: orders.StatusDelivered, wantErr: orders.ErrInvalidTransition},
		{name: "mart preparing", kind: orders.TypeMart, from: orders.StatusMerchantAccepted, to: orders.StatusPreparing},
		{name: "errand offer", kind: orders.TypeErrand, from: orders.StatusPlaced, to: orders.StatusRiderOffered},
		{name: "errand preparing", kind: orders.TypeErrand, from: orders.StatusPlaced, to: orders.StatusPreparing, wantErr: orders.ErrInvalidTransition},
		{name: "courier pickup", kind: orders.TypeCourier, from: orders.StatusAccepted, to: orders.StatusPickedUp},
		{name: "stay offered", kind: orders.TypeFood, from: orders.StatusRiderOffered, to: orders.StatusRiderOffered, wantErr: orders.ErrInvalidTransition},
		{name: "delivered stuck", kind: orders.TypeFood, from: orders.StatusDelivered, to: orders.StatusCancelled, wantErr: orders.ErrInvalidTransition},
		{name: "unknown type", kind: "ride", from: orders.StatusPlaced, to: orders.StatusCancelled, wantErr: orders.ErrUnknownType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := orders.Transition(tt.kind, tt.from, tt.to)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRequiresMerchant(t *testing.T) {
	if !orders.RequiresMerchant(orders.TypeFood) || !orders.RequiresMerchant(orders.TypeMart) {
		t.Fatal("catalog orders require a merchant")
	}
	if orders.RequiresMerchant(orders.TypeCourier) || orders.RequiresMerchant(orders.TypeErrand) {
		t.Fatal("direct tasks have no merchant")
	}
}

func TestEffortBPS(t *testing.T) {
	tests := []struct {
		tier    orders.EffortTier
		want    int64
		wantErr error
	}{
		{tier: orders.EffortLow, want: 10000},
		{tier: orders.EffortMedium, want: 13000},
		{tier: orders.EffortHigh, want: 16000},
		{tier: "urgent", wantErr: orders.ErrUnknownEffort},
	}
	for _, tt := range tests {
		got, err := orders.EffortBPS(tt.tier)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v", err)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if got != tt.want {
			t.Fatalf("bps = %d", got)
		}
	}
}

func TestTerminalsHaveNoExit(t *testing.T) {
	want := map[orders.Status]bool{
		orders.StatusDelivered: true,
		orders.StatusCancelled: true,
		orders.StatusRejected:  true,
	}
	for _, status := range orders.Statuses() {
		if orders.IsTerminal(status) != want[status] {
			t.Fatalf("terminal flag for %s", status)
		}
	}
}
