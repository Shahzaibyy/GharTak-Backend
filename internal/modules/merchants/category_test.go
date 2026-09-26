package merchants_test

import (
	"testing"

	"github.com/yourusername/ghartak-backend/internal/modules/merchants"
)

func TestDefaultCommission(t *testing.T) {
	tests := []struct {
		category merchants.Category
		want     string
		ok       bool
	}{
		{category: merchants.CategoryRestaurant, want: "18.00", ok: true},
		{category: merchants.CategoryMart, want: "10.00", ok: true},
		{category: merchants.CategoryPharmacy, want: "10.00", ok: true},
		{category: "grocery", ok: false},
	}
	for _, tt := range tests {
		got, ok := merchants.DefaultCommissionPercent(tt.category)
		if ok != tt.ok || got != tt.want {
			t.Fatalf("%s got %s %v", tt.category, got, ok)
		}
	}
}
