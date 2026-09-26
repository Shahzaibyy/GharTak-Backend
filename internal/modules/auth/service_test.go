package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/pii"
)

func TestVerifyOutcomes(t *testing.T) {
	phone := "03001234567"
	customerID := uuid.MustParse("33333333-3333-4333-8333-333333333301")
	tests := []struct {
		name    string
		role    Role
		otp     string
		seed    map[Role]string
		wantErr error
		message string
	}{
		{name: "match", role: RoleCustomer, otp: "123456", seed: map[Role]string{RoleCustomer: "123456"}},
		{name: "wrong code", role: RoleCustomer, otp: "000000", seed: map[Role]string{RoleCustomer: "123456"}, wantErr: apperror.ErrUnauthorized, message: "otp is incorrect"},
		{name: "expired", role: RoleCustomer, otp: "123456", wantErr: apperror.ErrUnauthorized, message: "otp expired"},
		{name: "wrong role", role: RoleRider, otp: "123456", seed: map[Role]string{RoleCustomer: "123456"}, wantErr: apperror.ErrUnauthorized, message: "different role"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, codes := verifyFixture(t, phone, customerID, tt.seed)
			session, err := svc.Verify(context.Background(), VerifyRequest{Phone: phone, Role: tt.role, OTP: tt.otp})
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) || !strings.Contains(err.Error(), tt.message) {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if session.AccountID != customerID || session.AccessToken == "" || len(codes.values) != 0 {
				t.Fatalf("session = %+v remaining = %d", session, len(codes.values))
			}
		})
	}
}

func verifyFixture(t *testing.T, phone string, customerID uuid.UUID, seed map[Role]string) (*Service, *fakeCodes) {
	t.Helper()
	lookup := lookupOf(t, phone)
	codes := &fakeCodes{values: map[string]string{}}
	for role, otp := range seed {
		codes.values[otpKey(role, lookup)] = hashOTP(jwtKey(), otp)
	}
	accounts := &fakeAccounts{customer: Account{ID: customerID, Status: "active"}}
	svc := NewService(accounts, codes, allowAll{}, &fakeSessions{}, piiKey(), hashKey(), jwtKey(), true)
	return svc, codes
}

func lookupOf(t *testing.T, phone string) string {
	t.Helper()
	e164, err := pii.NormalizePhone(phone)
	if err != nil {
		t.Fatal(err)
	}
	lookup, err := pii.PhoneLookup(hashKey(), e164)
	if err != nil {
		t.Fatal(err)
	}
	return lookup
}

func piiKey() []byte  { return bytes32("pii-key-for-auth-service-test!") }
func hashKey() []byte { return bytes32("hash-key-for-auth-service-tes!") }
func jwtKey() []byte  { return bytes32("jwt-key-for-auth-service-test!") }

func bytes32(raw string) []byte {
	buf := make([]byte, 32)
	copy(buf, raw)
	return buf
}

type fakeCodes struct {
	values map[string]string
}

func (f *fakeCodes) Save(_ context.Context, key, hash string, _ time.Duration) error {
	f.values[key] = hash
	return nil
}

func (f *fakeCodes) Load(_ context.Context, key string) (string, error) {
	hash, ok := f.values[key]
	if !ok {
		return "", apperror.ErrNotFound
	}
	return hash, nil
}

func (f *fakeCodes) Delete(_ context.Context, key string) error {
	delete(f.values, key)
	return nil
}

type allowAll struct{}

func (allowAll) Allow(context.Context, string, int, time.Duration) error { return nil }

type fakeAccounts struct {
	customer Account
}

func (f *fakeAccounts) UpsertCustomer(context.Context, string, string) (Account, error) {
	return f.customer, nil
}

func (f *fakeAccounts) Find(context.Context, Role, string) (Account, error) {
	return Account{}, apperror.ErrNotFound
}

type fakeSessions struct{}

func (fakeSessions) SaveRefresh(context.Context, string, refreshRecord, time.Duration) error {
	return nil
}

func (fakeSessions) Use(context.Context, string) (refreshRecord, error) {
	return refreshRecord{}, apperror.ErrNotFound
}

func (fakeSessions) MarkSpent(context.Context, string, string, time.Duration) error { return nil }

func (fakeSessions) SpentFamily(context.Context, string) (string, error) {
	return "", apperror.ErrNotFound
}

func (fakeSessions) RevokeFamily(context.Context, string) error { return nil }
