package schema_test

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/yourusername/ghartak-backend/internal/modules/admin"
	"github.com/yourusername/ghartak-backend/internal/modules/dispatch"
	"github.com/yourusername/ghartak-backend/internal/modules/merchants"
	"github.com/yourusername/ghartak-backend/internal/modules/orders"
	"github.com/yourusername/ghartak-backend/internal/modules/payments"
	"github.com/yourusername/ghartak-backend/internal/platform/domain"
)

func TestConstraintsMatchGo(t *testing.T) {
	body := readMigration(t, "0001_init.up.sql")
	tests := []struct {
		name       string
		constraint string
		want       []string
	}{
		{name: "user status", constraint: "users_status_check", want: asStrings(domain.AccountStatuses())},
		{name: "admin status", constraint: "admins_status_check", want: asStrings(domain.AccountStatuses())},
		{name: "rider verification", constraint: "riders_verification_status_check", want: asStrings(domain.VerificationStatuses())},
		{name: "merchant verification", constraint: "merchants_verification_status_check", want: asStrings(domain.VerificationStatuses())},
		{name: "merchant category", constraint: "merchants_category_check", want: asStrings(merchants.Categories())},
		{name: "order type", constraint: "orders_type_check", want: asStrings(orders.Types())},
		{name: "order status", constraint: "orders_status_check", want: asStrings(orders.Statuses())},
		{name: "effort", constraint: "orders_effort_tier_check", want: asStrings(orders.EffortTiers())},
		{name: "order payment", constraint: "orders_payment_method_check", want: asStrings(payments.Methods())},
		{name: "payment method", constraint: "payments_method_check", want: asStrings(payments.Methods())},
		{name: "payment status", constraint: "payments_status_check", want: asStrings(payments.Statuses())},
		{name: "ledger kind", constraint: "ledger_account_kind_check", want: asStrings(payments.AccountKinds())},
		{name: "ledger direction", constraint: "ledger_direction_check", want: asStrings(payments.Directions())},
		{name: "ledger entry", constraint: "ledger_entry_type_check", want: asStrings(payments.EntryTypes())},
		{name: "fraud", constraint: "fraud_flags_status_check", want: asStrings(domain.FraudStatuses())},
		{name: "event actor", constraint: "order_events_actor_role_check", want: asStrings(domain.EventActorRoles())},
		{name: "rater", constraint: "ratings_rater_role_check", want: asStrings(domain.RatingRoles())},
		{name: "ratee", constraint: "ratings_ratee_role_check", want: asStrings(domain.RatingRoles())},
		{name: "offer", constraint: "order_offers_response_check", want: asStrings(dispatch.OfferResponses())},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			equalSet(t, tt.constraint, constraintLiterals(t, body, tt.constraint), tt.want)
		})
	}
	if !strings.Contains(body, "score BETWEEN 1 AND 5") {
		t.Fatal("rating score check missing")
	}
}

func TestZoneSeedMatchesGo(t *testing.T) {
	body := readMigration(t, "0002_seed_zones.up.sql")
	for _, seed := range admin.ZoneSeeds() {
		line := lineContaining(t, body, seed.Slug)
		if !strings.Contains(line, seed.ID.String()) || !strings.Contains(line, seed.CityName) {
			t.Fatalf("seed line for %s = %s", seed.Slug, line)
		}
		if !strings.Contains(line, activeWord(seed.Active)) {
			t.Fatalf("active flag for %s", seed.Slug)
		}
	}
	for _, piece := range []string{admin.SeedBaseDeliveryFee, admin.SeedPerKmRate, admin.SeedSurge, admin.SeedRadiusKm} {
		if !strings.Contains(body, piece) {
			t.Fatalf("seed missing %s", piece)
		}
	}
}

func TestOnboardingConstraintsMatchGo(t *testing.T) {
	body := readMigration(t, "0009_onboarding_screens.up.sql")
	tests := []struct {
		name       string
		constraint string
		want       []string
	}{
		{name: "vehicle type", constraint: "riders_vehicle_type_check", want: asStrings(domain.VehicleTypes())},
		{name: "orientation", constraint: "riders_orientation_status_check", want: asStrings(domain.OrientationStatuses())},
		{name: "onboarding step", constraint: "riders_onboarding_step_check", want: asStrings(domain.RiderOnboardingSteps())},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			equalSet(t, tt.constraint, nullableConstraintLiterals(t, body, tt.constraint), tt.want)
		})
	}
}

func nullableConstraintLiterals(t *testing.T, body, name string) []string {
	t.Helper()
	pattern := `(?s)CONSTRAINT ` + regexp.QuoteMeta(name) + `[\s\S]*?IN \((.*?)\)`
	match := regexp.MustCompile(pattern).FindStringSubmatch(body)
	if match == nil {
		t.Fatalf("constraint %s not found", name)
	}
	return quotedList(match[1])
}

func activeWord(active bool) string {
	if active {
		return "true"
	}
	return "false"
}

func constraintLiterals(t *testing.T, body, name string) []string {
	t.Helper()
	pattern := `(?s)CONSTRAINT ` + regexp.QuoteMeta(name) + ` CHECK \([a-z_]+ IN \((.*?)\)\)`
	match := regexp.MustCompile(pattern).FindStringSubmatch(body)
	if match == nil {
		t.Fatalf("constraint %s not found", name)
	}
	return quotedList(match[1])
}

func quotedList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, "'")
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func equalSet(t *testing.T, name string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s got %v want %v", name, got, want)
	}
	seen := make(map[string]struct{}, len(want))
	for _, value := range want {
		seen[value] = struct{}{}
	}
	for _, value := range got {
		if _, ok := seen[value]; !ok {
			t.Fatalf("%s unexpected %s", name, value)
		}
		delete(seen, value)
	}
	for value := range seen {
		t.Fatalf("%s missing %s", name, value)
	}
}

func lineContaining(t *testing.T, body, slug string) string {
	t.Helper()
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, "'"+slug+"'") {
			return line
		}
	}
	t.Fatalf("line for %s not found", slug)
	return ""
}

func readMigration(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", name)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func asStrings[T ~string](values []T) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = string(value)
	}
	return out
}
