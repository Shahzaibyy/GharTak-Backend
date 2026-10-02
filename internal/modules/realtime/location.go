package realtime

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/geo"
	"github.com/yourusername/ghartak-backend/internal/platform/mapbox"
)

type locationFrame struct {
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Heading float64 `json:"heading,omitempty"`
	Speed   float64 `json:"speed,omitempty"`
	TS      int64   `json:"ts,omitempty"`
	ETAMin  int     `json:"eta_min,omitempty"`
}

type etaRouter interface {
	Route(ctx context.Context, from, to mapbox.Point) (mapbox.Route, error)
}

func (h *Handler) UseETA(router etaRouter) {
	h.eta = router
}

func (h *Handler) publishLocation(ctx context.Context, orderID uuid.UUID, data []byte, session session) {
	frame, ok := parseLocationFrame(data)
	if !ok {
		return
	}
	if !h.throttle(ctx, orderID) {
		return
	}
	frame.ETAMin = h.etaMin(ctx, orderID, session, frame)
	raw, err := json.Marshal(frame)
	if err != nil {
		return
	}
	_ = h.redis.Set(ctx, lastPosKey(orderID), raw, time.Hour).Err()
	if err := h.redis.Publish(ctx, locationChannel(orderID), raw).Err(); err != nil {
		h.log.Error().Err(err).Str("order_id", orderID.String()).Msg("location publish")
	}
}

func (h *Handler) throttle(ctx context.Context, orderID uuid.UUID) bool {
	ok, err := h.redis.SetNX(ctx, throttleKey(orderID), "1", 3*time.Second).Result()
	if err != nil {
		return true
	}
	return ok
}

func (h *Handler) etaMin(ctx context.Context, orderID uuid.UUID, session session, frame locationFrame) int {
	if h.eta == nil {
		return 0
	}
	if !h.etaThrottle(ctx, orderID) {
		return h.cachedETA(ctx, orderID)
	}
	target := mapbox.Point{Lat: session.party.PickupLat, Lng: session.party.PickupLng}
	route, err := h.eta.Route(ctx, mapbox.Point{Lat: frame.Lat, Lng: frame.Lng}, target)
	if err != nil {
		return h.cachedETA(ctx, orderID)
	}
	minutes := (route.DurationS + 30) / 60
	_ = h.redis.Set(ctx, etaKey(orderID), minutes, time.Minute).Err()
	return minutes
}

func (h *Handler) etaThrottle(ctx context.Context, orderID uuid.UUID) bool {
	ok, err := h.redis.SetNX(ctx, etaThrottleKey(orderID), "1", 45*time.Second).Result()
	if err != nil {
		return true
	}
	return ok
}

func (h *Handler) cachedETA(ctx context.Context, orderID uuid.UUID) int {
	raw, err := h.redis.Get(ctx, etaKey(orderID)).Int()
	if err != nil {
		return 0
	}
	return raw
}

func (h *Handler) writeLastPosition(ctx context.Context, orderID uuid.UUID, write func(context.Context, []byte) error) {
	raw, err := h.redis.Get(ctx, lastPosKey(orderID)).Bytes()
	if err != nil || len(raw) == 0 {
		return
	}
	_ = write(ctx, raw)
}

func parseLocationFrame(data []byte) (locationFrame, bool) {
	var frame locationFrame
	if err := json.Unmarshal(data, &frame); err != nil {
		return locationFrame{}, false
	}
	if !geo.ValidCoord(frame.Lat, frame.Lng) {
		return locationFrame{}, false
	}
	if frame.TS > 0 && frame.TS > time.Now().Add(2*time.Minute).UnixMilli() {
		return locationFrame{}, false
	}
	return frame, true
}

func lastPosKey(orderID uuid.UUID) string     { return "order:" + orderID.String() + ":last_pos" }
func throttleKey(orderID uuid.UUID) string    { return "order:" + orderID.String() + ":loc_throttle" }
func etaKey(orderID uuid.UUID) string         { return "order:" + orderID.String() + ":eta" }
func etaThrottleKey(orderID uuid.UUID) string { return "order:" + orderID.String() + ":eta_throttle" }
