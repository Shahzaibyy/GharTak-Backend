package orders

import (
	"testing"

	"github.com/yourusername/ghartak-backend/internal/platform/money"
)

func TestPrice(t *testing.T) {
	base := money.FromPaisa(6000)
	perKm := money.FromPaisa(1800)
	medium := EffortMedium
	tests := []struct {
		name       string
		meters     int64
		effort     *EffortTier
		itemPaisa  int64
		commission string
		surge      string
		wantFee    int64
		wantComm   int64
		wantRider  int64
	}{
		{name: "food one and a half km", meters: 1500, itemPaisa: 45000, commission: "18.00", surge: "1.00", wantFee: 8700, wantComm: 8100, wantRider: 6960},
		{name: "half up per km", meters: 1, itemPaisa: 0, commission: "0.00", surge: "1.00", wantFee: 6002, wantComm: 0, wantRider: 4802},
		{name: "effort and surge", meters: 0, effort: &medium, itemPaisa: 0, commission: "0.00", surge: "1.50", wantFee: 11700, wantComm: 0, wantRider: 9360},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Price(RateCard{Base: base, PerKm: perKm, Surge: tt.surge}, tt.meters, tt.effort, money.FromPaisa(tt.itemPaisa), tt.commission)
			if err != nil {
				t.Fatal(err)
			}
			if got.DeliveryFee.Paisa() != tt.wantFee || got.Commission.Paisa() != tt.wantComm || got.RiderEarning.Paisa() != tt.wantRider {
				t.Fatalf("priced = %+v", got)
			}
		})
	}
}

func TestHalfUpBoundary(t *testing.T) {
	one := money.FromPaisa(1)
	got, err := one.MulDiv(5000, 10000)
	if err != nil {
		t.Fatal(err)
	}
	if got.Paisa() != 1 {
		t.Fatalf("half up = %d", got.Paisa())
	}
	below, err := one.MulDiv(4999, 10000)
	if err != nil {
		t.Fatal(err)
	}
	if below.Paisa() != 0 {
		t.Fatalf("below half = %d", below.Paisa())
	}
}
