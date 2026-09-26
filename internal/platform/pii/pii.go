package pii

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"regexp"
	"strings"
)

var (
	ErrInvalidPhone      = errors.New("invalid phone")
	ErrInvalidKey        = errors.New("invalid key")
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
)

var mobilePK = regexp.MustCompile(`^\+923[0-9]{9}$`)

// NormalizePhone accepts local 03..., 92..., and +92... mobile numbers.
func NormalizePhone(raw string) (string, error) {
	compact := stripPhone(raw)
	if compact == "" {
		return "", ErrInvalidPhone
	}
	withCountry, err := ensureCountry(compact)
	if err != nil {
		return "", err
	}
	if !mobilePK.MatchString(withCountry) {
		return "", ErrInvalidPhone
	}
	return withCountry, nil
}

// PhoneLookup is the lowercase HMAC-SHA256 hex stored in phone_lookup columns.
func PhoneLookup(key []byte, e164 string) (string, error) {
	if len(key) != 32 {
		return "", ErrInvalidKey
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(e164))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// SealPhone normalizes, hashes, and encrypts one phone. Auth writes both returned values.
func SealPhone(encKey, hashKey []byte, raw string) (string, string, error) {
	e164, err := NormalizePhone(raw)
	if err != nil {
		return "", "", err
	}
	lookup, err := PhoneLookup(hashKey, e164)
	if err != nil {
		return "", "", err
	}
	ciphertext, err := Encrypt(encKey, e164)
	if err != nil {
		return "", "", err
	}
	return ciphertext, lookup, nil
}

func Encrypt(key []byte, plaintext string) (string, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	nonce, err := randomNonce(gcm.NonceSize())
	if err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func Decrypt(key []byte, encoded string) (string, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	raw, err := decodeCipher(encoded, gcm.NonceSize())
	if err != nil {
		return "", err
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", ErrInvalidCiphertext
	}
	return string(plain), nil
}

func decodeCipher(encoded string, nonceSize int) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(raw) < nonceSize {
		return nil, ErrInvalidCiphertext
	}
	return raw, nil
}

func stripPhone(raw string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(raw) {
		if r == ' ' || r == '-' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func ensureCountry(compact string) (string, error) {
	if strings.HasPrefix(compact, "+") {
		return compact, nil
	}
	if strings.HasPrefix(compact, "92") {
		return "+" + compact, nil
	}
	return localToE164(compact)
}

func localToE164(compact string) (string, error) {
	if strings.HasPrefix(compact, "0") && len(compact) > 1 {
		return "+92" + compact[1:], nil
	}
	return "", ErrInvalidPhone
}

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, ErrInvalidKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func randomNonce(size int) ([]byte, error) {
	nonce := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return nonce, nil
}
