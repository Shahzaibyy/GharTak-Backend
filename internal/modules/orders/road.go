package orders

import (
	"context"
	"errors"
	"strconv"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/modules/admin"
	"github.com/yourusername/ghartak-backend/internal/modules/merchants"
	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/geo"
	"github.com/yourusername/ghartak-backend/internal/platform/mapbox"
	"github.com/yourusername/ghartak-backend/internal/platform/money"
)

type roadRouter interface {
	Route(ctx context.Context, from, to mapbox.Point) (mapbox.Route, error)
}

func (s *Service) UseRouter(router roadRouter) *Service {
	s.router = router
	return s
}

func (s *Service) road(ctx context.Context, from, to mapbox.Point) (mapbox.Route, error) {
	if s.router == nil {
		meters := geo.RoadEstimate(geo.Point{Lat: from.Lat, Lng: from.Lng}, geo.Point{Lat: to.Lat, Lng: to.Lng}, 1.3)
		return mapbox.Route{DistanceM: int(meters), DurationS: geo.DurationSeconds(meters, 20), Approximate: true}, nil
	}
	return s.router.Route(ctx, from, to)
}

func (s *Service) quoteDraft(ctx context.Context, customerID uuid.UUID, in PlaceInput, zone admin.Zone, pin *merchants.Pin, itemTotal money.Money, items []OrderItem, commission string) (draft, error) {
	lat, lng, address := pickupOf(in, pin)
	if err := checkServiceArea(zone, lat, lng, in.DropLat, in.DropLng); err != nil {
		return draft{}, err
	}
	route, err := s.road(ctx, mapbox.Point{Lat: lat, Lng: lng}, mapbox.Point{Lat: in.DropLat, Lng: in.DropLng})
	if err != nil {
		return draft{}, mapRouteErr(err)
	}
	priced, err := Price(cardOf(zone), int64(route.DistanceM), in.Effort, itemTotal, commission)
	if err != nil {
		return draft{}, priceErr(err)
	}
	priced = withRoute(priced, route.DurationS, route.Approximate, route.Geometry)
	return draft{
		CustomerID: customerID, Input: in, PickupLat: lat, PickupLng: lng,
		PickupAddress: address, Items: items, Price: priced,
	}, nil
}

func checkServiceArea(zone admin.Zone, pickupLat, pickupLng, dropLat, dropLng float64) error {
	radius, err := strconv.ParseFloat(zone.ServiceRadiusKm, 64)
	if err != nil || radius <= 0 {
		return apperror.Invalid("zone radius is invalid")
	}
	center := geo.Point{Lat: zone.CenterLat, Lng: zone.CenterLng}
	if !geo.InRadius(center, geo.Point{Lat: pickupLat, Lng: pickupLng}, radius) {
		return apperror.Invalid("pickup is out of service area")
	}
	if !geo.InRadius(center, geo.Point{Lat: dropLat, Lng: dropLng}, radius) {
		return apperror.Invalid("drop is out of service area")
	}
	return nil
}

func mapRouteErr(err error) error {
	if errors.Is(err, mapbox.ErrNoRoute) || errors.Is(err, mapbox.ErrBadInput) {
		return apperror.Invalid("route is unavailable for these points")
	}
	return err
}
