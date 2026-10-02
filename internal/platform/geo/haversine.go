package geo

import "math"

// Point is a WGS84 coordinate. Coordinates are not money; float64 is fine.
type Point struct {
	Lat float64
	Lng float64
}

const earthMeters = 6371000.0

// HaversineMeters returns great-circle distance in meters, rounded half-up.
func HaversineMeters(from, to Point) int64 {
	p1 := from.Lat * math.Pi / 180
	p2 := to.Lat * math.Pi / 180
	dLat := (to.Lat - from.Lat) * math.Pi / 180
	dLng := (to.Lng - from.Lng) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return int64(math.Round(earthMeters * c))
}

// RoadEstimate applies a circuity factor to haversine for Mapbox fallback.
func RoadEstimate(from, to Point, circuity float64) int64 {
	if circuity < 1 {
		circuity = 1
	}
	straight := float64(HaversineMeters(from, to))
	return int64(math.Round(straight * circuity))
}

// DurationSeconds estimates travel time at a constant km/h.
func DurationSeconds(meters int64, kmh float64) int {
	if kmh <= 0 {
		kmh = 20
	}
	hours := (float64(meters) / 1000) / kmh
	return int(math.Round(hours * 3600))
}

// InRadius reports whether p is within radiusKm of center (haversine).
func InRadius(center, p Point, radiusKm float64) bool {
	if radiusKm <= 0 {
		return false
	}
	return float64(HaversineMeters(center, p)) <= radiusKm*1000
}

// ValidCoord checks WGS84 bounds.
func ValidCoord(lat, lng float64) bool {
	return lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180
}

// Round4 rounds to 4 decimal places (~11 m grid) for cache keys.
func Round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

// Round2 rounds to 2 decimal places for search proximity keys.
func Round2(v float64) float64 {
	return math.Round(v*100) / 100
}
