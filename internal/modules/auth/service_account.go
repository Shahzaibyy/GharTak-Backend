package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/money"
	"github.com/yourusername/ghartak-backend/internal/platform/notify"
	"github.com/yourusername/ghartak-backend/internal/platform/pii"
)

func (s *Service) GoogleSignIn(ctx context.Context, idToken string) (Session, error) {
	ident, err := s.verifyIdentity(ctx, idToken)
	if err != nil {
		return Session{}, err
	}
	return s.googleSession(ctx, ident)
}

func (s *Service) verifyIdentity(ctx context.Context, idToken string) (Identity, error) {
	if s.identity == nil {
		return Identity{}, apperror.ErrUnavailable
	}
	return s.identity.Verify(ctx, idToken)
}

func (s *Service) googleSession(ctx context.Context, ident Identity) (Session, error) {
	account, err := s.accounts.FindByFirebase(ctx, ident.UID)
	if errors.Is(err, apperror.ErrNotFound) {
		return s.createGoogle(ctx, ident)
	}
	if err != nil {
		return Session{}, err
	}
	return s.issueActive(ctx, account)
}

func (s *Service) createGoogle(ctx context.Context, ident Identity) (Session, error) {
	account, err := s.accounts.InsertGoogle(ctx, googleInsert{
		UID:   ident.UID,
		Email: optionalText(ident.Email, 320),
		Name:  optionalText(ident.Name, 100),
	})
	if err != nil {
		return Session{}, err
	}
	return s.issueActive(ctx, account)
}

func (s *Service) issueActive(ctx context.Context, account Account) (Session, error) {
	if err := allowStatus(account.Status); err != nil {
		return Session{}, err
	}
	return s.issue(ctx, account, RoleCustomer, uuid.NewString())
}

func (s *Service) Logout(ctx context.Context, raw string) error {
	return s.sessions.Revoke(ctx, hashToken(raw))
}

func (s *Service) Profile(ctx context.Context, id uuid.UUID) (Profile, error) {
	row, err := s.accounts.Profile(ctx, id)
	if err != nil {
		return Profile{}, err
	}
	return s.presentProfile(row)
}

func (s *Service) UpdateName(ctx context.Context, id uuid.UUID, name string) (Profile, error) {
	row, err := s.accounts.UpdateName(ctx, id, name)
	if err != nil {
		return Profile{}, err
	}
	return s.presentProfile(row)
}

func (s *Service) DeleteMe(ctx context.Context, id uuid.UUID) error {
	return s.accounts.SoftDelete(ctx, id)
}

func (s *Service) PhoneLink(ctx context.Context, userID uuid.UUID, phone, ip string) (OTPResult, error) {
	sealed, err := s.seal(phone)
	if err != nil {
		return OTPResult{}, err
	}
	if err := s.allow(ctx, ip, sealed.lookup); err != nil {
		return OTPResult{}, err
	}
	return s.storeLink(ctx, userID, sealed)
}

func (s *Service) storeLink(ctx context.Context, userID uuid.UUID, sealed sealedPhone) (OTPResult, error) {
	if err := s.codes.Save(ctx, pendingPhoneKey(userID), packPhone(sealed), otpTTL); err != nil {
		return OTPResult{}, err
	}
	plain, err := pii.Decrypt(s.piiKey, sealed.ciphertext)
	if err != nil {
		return OTPResult{}, err
	}
	return s.issueOTP(ctx, RoleCustomer, linkSubject(userID), notify.ChannelWhatsApp, plain)
}

func (s *Service) PhoneLinkVerify(ctx context.Context, userID uuid.UUID, otp string) error {
	if err := s.checkOTP(ctx, RoleCustomer, linkSubject(userID), otp); err != nil {
		return err
	}
	return s.attachPending(ctx, userID)
}

func (s *Service) attachPending(ctx context.Context, userID uuid.UUID) error {
	raw, err := s.codes.Load(ctx, pendingPhoneKey(userID))
	if err != nil {
		return err
	}
	sealed, err := unpackPhone(raw)
	if err != nil {
		return err
	}
	if err := s.accounts.AttachPhone(ctx, userID, sealed.ciphertext, sealed.lookup); err != nil {
		return err
	}
	return s.codes.Delete(ctx, pendingPhoneKey(userID))
}

func (s *Service) RequirePhone(ctx context.Context, id uuid.UUID) error {
	verified, err := s.accounts.PhoneVerified(ctx, id)
	if err != nil {
		return err
	}
	return phoneRequired(verified)
}

func (s *Service) RequestEmailOTP(ctx context.Context, req EmailOTPRequest) (OTPResult, error) {
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return OTPResult{}, err
	}
	if err := s.allowEmail(ctx, req.IP, email); err != nil {
		return OTPResult{}, err
	}
	return s.issueOTP(ctx, RoleCustomer, emailSubject(email), notify.ChannelEmail, email)
}

func (s *Service) VerifyEmail(ctx context.Context, req EmailVerifyRequest) (Session, error) {
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return Session{}, err
	}
	if err := s.checkOTP(ctx, RoleCustomer, emailSubject(email), req.OTP); err != nil {
		return Session{}, err
	}
	account, err := s.emailAccount(ctx, email)
	if err != nil {
		return Session{}, err
	}
	return s.issueActive(ctx, account)
}

func (s *Service) emailAccount(ctx context.Context, email string) (Account, error) {
	account, err := s.accounts.FindByEmail(ctx, email)
	if errors.Is(err, apperror.ErrNotFound) {
		return s.accounts.InsertEmail(ctx, email)
	}
	return account, err
}

func (s *Service) SetPreferences(ctx context.Context, id uuid.UUID, types []string) (Profile, error) {
	row, err := s.accounts.SetPreferences(ctx, id, types)
	if err != nil {
		return Profile{}, err
	}
	return s.presentProfile(row)
}

func (s *Service) allowEmail(ctx context.Context, ip, email string) error {
	// OTP rate limit disabled for demo/testing — re-enable when needed:
	// if err := s.limits.Allow(ctx, "rl:otp:ip:"+ip, ipLimit, time.Hour); err != nil {
	// 	return err
	// }
	// return s.limits.Allow(ctx, "rl:otp:email:"+email, phoneLimit, time.Hour)
	_ = ctx
	_ = ip
	_ = email
	return nil
}

func normalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if len(email) < 5 || len(email) > 320 || !strings.Contains(email, "@") {
		return "", apperror.Invalid("email is invalid")
	}
	return email, nil
}

func emailSubject(email string) string { return "email:" + email }

func phoneRequired(verified bool) error {
	if verified {
		return nil
	}
	return apperror.ConflictCode("phone_required", "verify a phone number before placing an order")
}

func (s *Service) presentProfile(row profileRow) (Profile, error) {
	wallet, err := money.Parse(row.Wallet)
	if err != nil {
		return Profile{}, fmt.Errorf("auth: wallet: %w", err)
	}
	phone, err := s.openPhone(row.PhoneCipher)
	if err != nil {
		return Profile{}, err
	}
	return Profile{
		ID: row.ID, Name: row.Name, Email: row.Email, Phone: phone,
		PhoneVerified: row.PhoneVerified, WalletBalance: wallet,
		PreferredOrderTypes: row.PreferredOrderTypes,
	}, nil
}

func (s *Service) openPhone(cipher *string) (*string, error) {
	if cipher == nil || *cipher == "" {
		return nil, nil
	}
	plain, err := pii.Decrypt(s.piiKey, *cipher)
	if err != nil {
		return nil, fmt.Errorf("auth: open phone: %w", err)
	}
	return &plain, nil
}

func optionalText(raw string, max int) *string {
	text := strings.TrimSpace(raw)
	if text == "" || len(text) > max {
		return nil
	}
	return &text
}

func packPhone(sealed sealedPhone) string {
	return sealed.ciphertext + "\n" + sealed.lookup
}

func unpackPhone(raw string) (sealedPhone, error) {
	cipher, lookup, ok := strings.Cut(raw, "\n")
	if !ok || cipher == "" || lookup == "" {
		return sealedPhone{}, apperror.ErrNotFound
	}
	return sealedPhone{ciphertext: cipher, lookup: lookup}, nil
}

func linkSubject(id uuid.UUID) string { return "link:" + id.String() }

func pendingPhoneKey(id uuid.UUID) string { return "pending-phone:" + id.String() }

func AccountID(ctx context.Context) (uuid.UUID, error) {
	principal, ok := PrincipalFrom(ctx)
	if !ok {
		return uuid.UUID{}, apperror.ErrUnauthorized
	}
	return principal.AccountID, nil
}
