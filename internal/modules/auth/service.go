package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/pii"
)

const (
	otpTTL     = 5 * time.Minute
	refreshTTL = 30 * 24 * time.Hour
	phoneLimit = 5
	ipLimit    = 20
)

type sealedPhone struct {
	ciphertext string
	lookup     string
}

type OTPResult struct {
	DevOTP string `json:"dev_otp,omitempty"`
}

type codeStore interface {
	Save(ctx context.Context, key, hash string, ttl time.Duration) error
	Load(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
}

type limiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) error
}

type sessionStore interface {
	SaveRefresh(ctx context.Context, hash string, record refreshRecord, ttl time.Duration) error
	Use(ctx context.Context, hash string) (refreshRecord, error)
	MarkSpent(ctx context.Context, hash, familyID string, ttl time.Duration) error
	SpentFamily(ctx context.Context, hash string) (string, error)
	RevokeFamily(ctx context.Context, familyID string) error
	Revoke(ctx context.Context, hash string) error
}

type accountStore interface {
	UpsertCustomer(ctx context.Context, ciphertext, lookup string) (Account, error)
	Find(ctx context.Context, role Role, lookup string) (Account, error)
	FindByFirebase(ctx context.Context, uid string) (Account, error)
	InsertGoogle(ctx context.Context, row googleInsert) (Account, error)
	Profile(ctx context.Context, id uuid.UUID) (profileRow, error)
	UpdateName(ctx context.Context, id uuid.UUID, name string) (profileRow, error)
	AttachPhone(ctx context.Context, id uuid.UUID, ciphertext, lookup string) error
	PhoneVerified(ctx context.Context, id uuid.UUID) (bool, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

type tokenVerifier interface {
	Verify(ctx context.Context, idToken string) (Identity, error)
}

type Service struct {
	accounts accountStore
	codes    codeStore
	limits   limiter
	sessions sessionStore
	piiKey   []byte
	hashKey  []byte
	jwtKey   []byte
	dev      bool
	identity tokenVerifier
	now      func() time.Time
}

func NewService(accounts accountStore, codes codeStore, limits limiter, sessions sessionStore, piiKey, hashKey, jwtKey []byte, dev bool) *Service {
	return &Service{
		accounts: accounts,
		codes:    codes,
		limits:   limits,
		sessions: sessions,
		piiKey:   piiKey,
		hashKey:  hashKey,
		jwtKey:   jwtKey,
		dev:      dev,
		now:      time.Now,
	}
}

func (s *Service) UseIdentity(verifier tokenVerifier) {
	s.identity = verifier
}

func (s *Service) RequestOTP(ctx context.Context, req OTPRequest) (OTPResult, error) {
	sealed, err := s.seal(req.Phone)
	if err != nil {
		return OTPResult{}, err
	}
	if err := s.allow(ctx, req.IP, sealed.lookup); err != nil {
		return OTPResult{}, err
	}
	return s.issueOTP(ctx, req.Role, sealed.lookup)
}

func (s *Service) Verify(ctx context.Context, req VerifyRequest) (Session, error) {
	sealed, err := s.seal(req.Phone)
	if err != nil {
		return Session{}, err
	}
	return s.verifySealed(ctx, req, sealed)
}

func (s *Service) verifySealed(ctx context.Context, req VerifyRequest, sealed sealedPhone) (Session, error) {
	if err := s.checkOTP(ctx, req.Role, sealed.lookup, req.OTP); err != nil {
		return Session{}, err
	}
	account, err := s.accountFor(ctx, req.Role, sealed)
	if err != nil {
		return Session{}, err
	}
	if err := allowStatus(account.Status); err != nil {
		return Session{}, err
	}
	return s.issue(ctx, account, req.Role, uuid.NewString())
}

func (s *Service) Refresh(ctx context.Context, raw string) (Session, error) {
	hash := hashToken(raw)
	record, err := s.sessions.Use(ctx, hash)
	if errors.Is(err, apperror.ErrNotFound) {
		return s.reuse(ctx, hash)
	}
	if err != nil {
		return Session{}, err
	}
	if err := s.sessions.MarkSpent(ctx, hash, record.FamilyID, refreshTTL); err != nil {
		return Session{}, err
	}
	return s.rotate(ctx, record)
}

func (s *Service) seal(raw string) (sealedPhone, error) {
	ciphertext, lookup, err := pii.SealPhone(s.piiKey, s.hashKey, raw)
	if err != nil {
		return sealedPhone{}, apperror.Invalid("phone is invalid")
	}
	return sealedPhone{ciphertext: ciphertext, lookup: lookup}, nil
}

func (s *Service) allow(ctx context.Context, ip, lookup string) error {
	if err := s.limits.Allow(ctx, "rl:otp:ip:"+ip, ipLimit, time.Hour); err != nil {
		return err
	}
	return s.limits.Allow(ctx, "rl:otp:phone:"+lookup, phoneLimit, time.Hour)
}

func (s *Service) issueOTP(ctx context.Context, role Role, lookup string) (OTPResult, error) {
	code, err := newOTP()
	if err != nil {
		return OTPResult{}, err
	}
	if err := s.codes.Save(ctx, otpKey(role, lookup), hashOTP(s.jwtKey, code), otpTTL); err != nil {
		return OTPResult{}, err
	}
	return OTPResult{DevOTP: s.devCode(code)}, nil
}

func (s *Service) devCode(code string) string {
	if !s.dev {
		return ""
	}
	return code
}

func (s *Service) checkOTP(ctx context.Context, role Role, lookup, otp string) error {
	key := otpKey(role, lookup)
	stored, err := s.codes.Load(ctx, key)
	if errors.Is(err, apperror.ErrNotFound) {
		return s.missingOTP(ctx, role, lookup)
	}
	if err != nil {
		return err
	}
	if !otpMatches(s.jwtKey, otp, stored) {
		return apperror.Unauthorized("otp is incorrect")
	}
	return s.codes.Delete(ctx, key)
}

func (s *Service) missingOTP(ctx context.Context, role Role, lookup string) error {
	if s.otherRole(ctx, role, lookup) {
		return apperror.Unauthorized("otp was issued for a different role")
	}
	return apperror.Unauthorized("otp expired")
}

func (s *Service) otherRole(ctx context.Context, role Role, lookup string) bool {
	for _, candidate := range []Role{RoleCustomer, RoleRider, RoleMerchant, RoleAdmin} {
		if candidate == role {
			continue
		}
		if s.hasCode(ctx, candidate, lookup) {
			return true
		}
	}
	return false
}

func (s *Service) hasCode(ctx context.Context, role Role, lookup string) bool {
	_, err := s.codes.Load(ctx, otpKey(role, lookup))
	return err == nil
}

func (s *Service) accountFor(ctx context.Context, role Role, sealed sealedPhone) (Account, error) {
	if role == RoleCustomer {
		return s.accounts.UpsertCustomer(ctx, sealed.ciphertext, sealed.lookup)
	}
	account, err := s.accounts.Find(ctx, role, sealed.lookup)
	if errors.Is(err, apperror.ErrNotFound) {
		return Account{}, apperror.Unauthorized("account is not registered")
	}
	return account, err
}

func allowStatus(status string) error {
	if _, blocked := blockedStatus[status]; blocked {
		return apperror.Forbidden("account is not allowed to sign in")
	}
	return nil
}

var blockedStatus = map[string]struct{}{
	"suspended": {},
	"rejected":  {},
}

func (s *Service) issue(ctx context.Context, account Account, role Role, familyID string) (Session, error) {
	access, err := signAccess(s.jwtKey, account.ID, role, s.now())
	if err != nil {
		return Session{}, err
	}
	raw, hash, err := newRefresh()
	if err != nil {
		return Session{}, err
	}
	record := refreshRecord{AccountID: account.ID.String(), Role: string(role), Status: account.Status, FamilyID: familyID}
	if err := s.sessions.SaveRefresh(ctx, hash, record, refreshTTL); err != nil {
		return Session{}, err
	}
	return Session{
		AccessToken:  access,
		RefreshToken: raw,
		ExpiresIn:    int(accessTTL.Seconds()),
		AccountID:    account.ID,
		Role:         role,
		Status:       account.Status,
	}, nil
}

func (s *Service) reuse(ctx context.Context, hash string) (Session, error) {
	familyID, err := s.sessions.SpentFamily(ctx, hash)
	if errors.Is(err, apperror.ErrNotFound) {
		return Session{}, apperror.ErrUnauthorized
	}
	if err != nil {
		return Session{}, err
	}
	if err := s.sessions.RevokeFamily(ctx, familyID); err != nil {
		return Session{}, err
	}
	return Session{}, apperror.ErrUnauthorized
}

func (s *Service) rotate(ctx context.Context, record refreshRecord) (Session, error) {
	id, err := uuid.Parse(record.AccountID)
	if err != nil {
		return Session{}, apperror.ErrUnauthorized
	}
	role, ok := ParseRole(record.Role)
	if !ok {
		return Session{}, apperror.ErrUnauthorized
	}
	if err := allowStatus(record.Status); err != nil {
		return Session{}, err
	}
	return s.issue(ctx, Account{ID: id, Status: record.Status}, role, record.FamilyID)
}
