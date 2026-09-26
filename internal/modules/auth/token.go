package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
)

const accessTTL = time.Hour

type Principal struct {
	AccountID uuid.UUID
	Role      Role
}

type accessClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func signAccess(key []byte, accountID uuid.UUID, role Role, now time.Time) (string, error) {
	claims := accessClaims{
		Role: string(role),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   accountID.String(),
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(key)
	if err != nil {
		return "", apperror.ErrUnauthorized
	}
	return signed, nil
}

func ParseAccess(key []byte, header string) (Principal, error) {
	raw, err := bearer(header)
	if err != nil {
		return Principal{}, err
	}
	claims := &accessClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, hmacKey(key))
	if err != nil || !token.Valid {
		return Principal{}, apperror.ErrUnauthorized
	}
	return principalFrom(claims)
}

func hmacKey(key []byte) jwt.Keyfunc {
	return func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, apperror.ErrUnauthorized
		}
		return key, nil
	}
}

func bearer(header string) (string, error) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) || len(header) <= len(prefix) {
		return "", apperror.ErrUnauthorized
	}
	return strings.TrimSpace(header[len(prefix):]), nil
}

func principalFrom(claims *accessClaims) (Principal, error) {
	role, ok := ParseRole(claims.Role)
	if !ok {
		return Principal{}, apperror.ErrUnauthorized
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return Principal{}, apperror.ErrUnauthorized
	}
	return Principal{AccountID: id, Role: role}, nil
}

func newRefresh() (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	raw := hex.EncodeToString(buf)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
