package pii

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

var ErrInvalidCNIC = errors.New("invalid cnic")

// NormalizeCNIC accepts 12345-1234567-1 and the 13 digits with no dashes.
func NormalizeCNIC(raw string) (string, error) {
	compact := compactCNIC(raw)
	if len(compact) != 13 || !digits(compact) {
		return "", ErrInvalidCNIC
	}
	return compact, nil
}

// SealCNIC hashes and encrypts a CNIC. The lookup is HMAC("cnic:"+digits).
func SealCNIC(encKey, hashKey []byte, raw string) (string, string, error) {
	digits, err := NormalizeCNIC(raw)
	if err != nil {
		return "", "", err
	}
	lookup, err := identityLookup(hashKey, "cnic:"+digits)
	if err != nil {
		return "", "", err
	}
	ciphertext, err := Encrypt(encKey, digits)
	if err != nil {
		return "", "", err
	}
	return ciphertext, lookup, nil
}

func compactCNIC(raw string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(raw) {
		if r == '-' || r == ' ' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func digits(raw string) bool {
	for _, r := range raw {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func identityLookup(key []byte, value string) (string, error) {
	if len(key) != 32 {
		return "", ErrInvalidKey
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil)), nil
}
