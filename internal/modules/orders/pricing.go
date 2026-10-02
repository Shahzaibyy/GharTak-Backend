package orders

import (
	"encoding/json"
	"math"

	"github.com/yourusername/ghartak-backend/internal/modules/payments"
	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/money"
)

type RateCard struct {
	Base  money.Money
	PerKm money.Money
	Surge string
}

type Line struct {
	Price money.Money
	Qty   int64
}

type Priced struct {
	DistanceKm      string          `json:"distance_km"`
	DurationMin     int             `json:"duration_min,omitempty"`
	ItemTotal       money.Money     `json:"item_total"`
	DeliveryFee     money.Money     `json:"delivery_fee"`
	Commission      money.Money     `json:"commission_amount"`
	RiderEarning    money.Money     `json:"rider_earning"`
	SurgeMultiplier string          `json:"surge_multiplier"`
	Total           money.Money     `json:"total"`
	Approximate     bool            `json:"approximate"`
	Route           json.RawMessage `json:"route,omitempty"`
}

func Price(card RateCard, meters int64, effort *EffortTier, itemTotal money.Money, commissionPercent string) (Priced, error) {
	fee, err := deliveryFee(card, meters)
	if err != nil {
		return Priced{}, err
	}
	fee, err = applyFactors(fee, effort, card.Surge)
	if err != nil {
		return Priced{}, err
	}
	return finish(meters, itemTotal, fee, commissionPercent, card.Surge)
}

func ItemTotal(lines []Line) (money.Money, error) {
	total := money.FromPaisa(0)
	for _, line := range lines {
		next, err := addLine(total, line)
		if err != nil {
			return money.Money{}, err
		}
		total = next
	}
	return total, nil
}

func distanceMeters(lat1, lng1, lat2, lng2 float64) int64 {
	const earth = 6371000.0
	p1 := lat1 * math.Pi / 180
	p2 := lat2 * math.Pi / 180
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return int64(math.Round(earth * c))
}

func withRoute(priced Priced, durationS int, approximate bool, geometry json.RawMessage) Priced {
	priced.DurationMin = (durationS + 30) / 60
	priced.Approximate = approximate
	priced.Route = geometry
	return priced
}

func deliveryFee(card RateCard, meters int64) (money.Money, error) {
	variable, err := card.PerKm.MulDiv(meters, 1000)
	if err != nil {
		return money.Money{}, err
	}
	return card.Base.Add(variable)
}

func applyFactors(fee money.Money, effort *EffortTier, surge string) (money.Money, error) {
	factor, err := effortFactor(effort)
	if err != nil {
		return money.Money{}, err
	}
	scaled, err := fee.MulDiv(factor, 10000)
	if err != nil {
		return money.Money{}, err
	}
	return scaleSurge(scaled, surge)
}

func scaleSurge(fee money.Money, surge string) (money.Money, error) {
	bps, err := surgeBPS(surge)
	if err != nil {
		return money.Money{}, err
	}
	return fee.MulDiv(bps, 10000)
}

func finish(meters int64, itemTotal, fee money.Money, commissionPercent, surge string) (Priced, error) {
	commission, err := commissionOf(itemTotal, commissionPercent)
	if err != nil {
		return Priced{}, err
	}
	rider, err := fee.MulDiv(payments.RiderDeliveryShareBPS, 10000)
	if err != nil {
		return Priced{}, err
	}
	total, err := itemTotal.Add(fee)
	if err != nil {
		return Priced{}, err
	}
	return priced(meters, itemTotal, fee, commission, rider, surge, total)
}

func priced(meters int64, itemTotal, fee, commission, rider money.Money, surge string, total money.Money) (Priced, error) {
	if total.IsZero() || total.IsNegative() {
		return Priced{}, apperror.Invalid("order total must be greater than zero")
	}
	return Priced{
		DistanceKm: kmString(meters), ItemTotal: itemTotal, DeliveryFee: fee,
		Commission: commission, RiderEarning: rider, SurgeMultiplier: surge, Total: total,
	}, nil
}

func addLine(total money.Money, line Line) (money.Money, error) {
	part, err := line.Price.MulDiv(line.Qty, 1)
	if err != nil {
		return money.Money{}, err
	}
	return total.Add(part)
}

func effortFactor(tier *EffortTier) (int64, error) {
	if tier == nil {
		return 10000, nil
	}
	return EffortBPS(*tier)
}

func surgeBPS(raw string) (int64, error) {
	parsed, err := money.Parse(raw)
	if err != nil {
		return 0, err
	}
	return parsed.Paisa() * 100, nil
}

func commissionOf(item money.Money, percent string) (money.Money, error) {
	bps, err := percentBPS(percent)
	if err != nil {
		return money.Money{}, err
	}
	return item.MulDiv(bps, 10000)
}

func percentBPS(raw string) (int64, error) {
	parsed, err := money.Parse(raw)
	if err != nil {
		return 0, err
	}
	return parsed.Paisa(), nil
}

func kmString(meters int64) string {
	hundredths := (meters + 5) / 10
	return money.FromPaisa(hundredths).String()
}
