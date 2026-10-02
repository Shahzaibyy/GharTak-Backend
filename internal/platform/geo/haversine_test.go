package geo

import "testing"

func TestHaversineMeters(t *testing.T) {
	from := Point{Lat: 33.7667, Lng: 72.3667}
	to := Point{Lat: 33.8194, Lng: 72.6892}
	got := HaversineMeters(from, to)
	if got < 28000 || got > 35000 {
		t.Fatalf("attock to hasan abdal = %d", got)
	}
	if HaversineMeters(from, from) != 0 {
		t.Fatal("same point")
	}
}

func TestRoadEstimate(t *testing.T) {
	from := Point{Lat: 0, Lng: 0}
	to := Point{Lat: 0, Lng: 0.01}
	straight := HaversineMeters(from, to)
	road := RoadEstimate(from, to, 1.3)
	if road < straight || road > straight*2 {
		t.Fatalf("straight=%d road=%d", straight, road)
	}
}

func TestInRadius(t *testing.T) {
	center := Point{Lat: 33.7667, Lng: 72.3667}
	inside := Point{Lat: 33.7800, Lng: 72.3800}
	outside := Point{Lat: 34.2000, Lng: 72.9000}
	if !InRadius(center, inside, 8) {
		t.Fatal("inside expected")
	}
	if InRadius(center, outside, 8) {
		t.Fatal("outside expected")
	}
}

func TestRoundGrid(t *testing.T) {
	if Round4(33.76675) != 33.7668 {
		t.Fatalf("round4 = %v", Round4(33.76675))
	}
	if Round2(72.3667) != 72.37 {
		t.Fatalf("round2 = %v", Round2(72.3667))
	}
}

func TestValidCoord(t *testing.T) {
	if !ValidCoord(33.7, 72.3) || ValidCoord(91, 0) || ValidCoord(0, 181) {
		t.Fatal("bounds")
	}
}

func TestDurationSeconds(t *testing.T) {
	// 20 km at 20 km/h = 1 hour
	if got := DurationSeconds(20000, 20); got != 3600 {
		t.Fatalf("duration = %d", got)
	}
}
