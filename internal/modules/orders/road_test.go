package orders

import (
	"testing"

	"github.com/yourusername/ghartak-backend/internal/modules/admin"
)

func TestCheckServiceArea(t *testing.T) {
	zone := admin.Zone{
		CenterLat: 33.7667, CenterLng: 72.3667, ServiceRadiusKm: "8.00",
	}
	if err := checkServiceArea(zone, 33.7800, 72.3800, 33.7700, 72.3700); err != nil {
		t.Fatalf("inside = %v", err)
	}
	if err := checkServiceArea(zone, 34.2000, 72.9000, 33.7700, 72.3700); err == nil {
		t.Fatal("pickup outside expected")
	}
	if err := checkServiceArea(zone, 33.7700, 72.3700, 34.2000, 72.9000); err == nil {
		t.Fatal("drop outside expected")
	}
}
