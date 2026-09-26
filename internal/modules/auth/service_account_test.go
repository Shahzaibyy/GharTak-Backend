package auth

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/pii"
)

func TestGoogleSignIn(t *testing.T) {
	returning := uuid.MustParse("55555555-5555-4555-8555-555555555501")
	tests := []struct {
		name       string
		ident      Identity
		verifyErr  error
		existing   Account
		wantInsert bool
		wantErr    error
		message    string
	}{
		{name: "new user", ident: Identity{UID: "firebase-new", Email: "ada@example.com", Name: "Ada"}, wantInsert: true},
		{name: "returning user", ident: Identity{UID: "firebase-old", Name: "Ada"}, existing: Account{ID: returning, Status: "active"}},
		{name: "expired", verifyErr: apperror.Unauthorized("firebase token expired"), wantErr: apperror.ErrUnauthorized, message: "expired"},
		{name: "wrong audience", verifyErr: apperror.Unauthorized("firebase token audience is invalid"), wantErr: apperror.ErrUnauthorized, message: "audience"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accounts := &memAccounts{existing: tt.existing}
			svc := accountService(accounts, &fakeCodes{values: map[string]string{}})
			svc.UseIdentity(stubIdentity{ident: tt.ident, err: tt.verifyErr})
			session, err := svc.GoogleSignIn(context.Background(), "token")
			assertGoogle(t, tt.wantErr, tt.message, tt.wantInsert, tt.existing.ID, accounts, session, err)
		})
	}
}

func TestProfileAndPhoneGate(t *testing.T) {
	id := uuid.MustParse("55555555-5555-4555-8555-555555555502")
	cipher, err := pii.Encrypt(piiKey(), "+923001234567")
	if err != nil {
		t.Fatal(err)
	}
	accounts := &memAccounts{profile: profileRow{
		ID: id, Name: strPtr("Ada"), Email: strPtr("ada@example.com"),
		PhoneCipher: &cipher, PhoneVerified: false, Wallet: "12.50",
	}}
	svc := accountService(accounts, &fakeCodes{values: map[string]string{}})
	profile, err := svc.Profile(context.Background(), id)
	if err != nil || profile.Phone == nil || *profile.Phone != "+923001234567" || profile.WalletBalance.String() != "12.50" {
		t.Fatalf("profile = %+v err = %v", profile, err)
	}
	updated, err := svc.UpdateName(context.Background(), id, "Nadia")
	if err != nil || updated.Name == nil || *updated.Name != "Nadia" {
		t.Fatalf("updated = %+v err = %v", updated, err)
	}
	requireErr := svc.RequirePhone(context.Background(), id)
	var coded *apperror.Error
	if !errors.Is(requireErr, apperror.ErrConflict) || !errors.As(requireErr, &coded) || coded.Code() != "phone_required" {
		t.Fatalf("require = %v", requireErr)
	}
	accounts.profile.PhoneVerified = true
	if err := svc.RequirePhone(context.Background(), id); err != nil {
		t.Fatal(err)
	}
}

func TestPhoneLink(t *testing.T) {
	id := uuid.MustParse("55555555-5555-4555-8555-555555555503")
	codes := &fakeCodes{values: map[string]string{}}
	accounts := &memAccounts{}
	svc := accountService(accounts, codes)
	result, err := svc.PhoneLink(context.Background(), id, "03001234567", "127.0.0.1")
	if err != nil || result.DevOTP == "" {
		t.Fatalf("link = %+v err = %v", result, err)
	}
	if err := svc.PhoneLinkVerify(context.Background(), id, "000000"); !errors.Is(err, apperror.ErrUnauthorized) {
		t.Fatalf("wrong otp = %v", err)
	}
	if err := svc.PhoneLinkVerify(context.Background(), id, result.DevOTP); err != nil {
		t.Fatal(err)
	}
	if accounts.phone == "" || !accounts.verified {
		t.Fatalf("attached = %q verified = %v", accounts.phone, accounts.verified)
	}
}

func TestCanDelete(t *testing.T) {
	tests := []struct {
		name    string
		state   userLock
		wantErr error
	}{
		{name: "balance", state: userLock{}, wantErr: apperror.ErrConflict},
		{name: "deleted", state: userLock{deleted: true, zero: true}, wantErr: apperror.ErrNotFound},
		{name: "ok", state: userLock{zero: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := canDelete(tt.state)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func assertGoogle(t *testing.T, wantErr error, message string, inserted bool, existing uuid.UUID, accounts *memAccounts, session Session, err error) {
	t.Helper()
	if wantErr != nil {
		if !errors.Is(err, wantErr) || (message != "" && !strings.Contains(err.Error(), message)) {
			t.Fatalf("err = %v", err)
		}
		return
	}
	if err != nil || session.Role != RoleCustomer || session.AccessToken == "" {
		t.Fatalf("session = %+v err = %v", session, err)
	}
	if accounts.inserted != inserted {
		t.Fatalf("inserted = %v", accounts.inserted)
	}
	if existing != uuid.Nil && session.AccountID != existing {
		t.Fatalf("account = %s", session.AccountID)
	}
}

func accountService(accounts *memAccounts, codes *fakeCodes) *Service {
	return NewService(accounts, codes, allowAll{}, &fakeSessions{}, piiKey(), hashKey(), jwtKey(), true)
}

func strPtr(value string) *string { return &value }

type stubIdentity struct {
	ident Identity
	err   error
}

func (s stubIdentity) Verify(context.Context, string) (Identity, error) {
	return s.ident, s.err
}

type memAccounts struct {
	existing Account
	profile  profileRow
	inserted bool
	phone    string
	verified bool
}

func (m *memAccounts) UpsertCustomer(context.Context, string, string) (Account, error) {
	return Account{}, apperror.ErrNotFound
}

func (m *memAccounts) Find(context.Context, Role, string) (Account, error) {
	return Account{}, apperror.ErrNotFound
}

func (m *memAccounts) FindByFirebase(_ context.Context, uid string) (Account, error) {
	if m.existing.ID == uuid.Nil || uid == "" {
		return Account{}, apperror.ErrNotFound
	}
	return m.existing, nil
}

func (m *memAccounts) InsertGoogle(context.Context, googleInsert) (Account, error) {
	m.inserted = true
	return Account{ID: uuid.MustParse("55555555-5555-4555-8555-555555555509"), Status: "active"}, nil
}

func (m *memAccounts) Profile(context.Context, uuid.UUID) (profileRow, error) {
	return m.profile, nil
}

func (m *memAccounts) UpdateName(_ context.Context, id uuid.UUID, name string) (profileRow, error) {
	m.profile.ID = id
	m.profile.Name = &name
	if m.profile.Wallet == "" {
		m.profile.Wallet = "0.00"
	}
	return m.profile, nil
}

func (m *memAccounts) AttachPhone(_ context.Context, _ uuid.UUID, _, lookup string) error {
	m.phone = lookup
	m.verified = true
	return nil
}

func (m *memAccounts) PhoneVerified(context.Context, uuid.UUID) (bool, error) {
	return m.profile.PhoneVerified, nil
}

func (m *memAccounts) SoftDelete(context.Context, uuid.UUID) error { return nil }
