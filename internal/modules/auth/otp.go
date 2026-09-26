package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

func newOTP() (string, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("auth: otp: %w", err)
	}
	n := binary.BigEndian.Uint32(buf[:]) % 1000000
	return fmt.Sprintf("%06d", n), nil
}

func hashOTP(key []byte, otp string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("otp:" + otp))
	return hex.EncodeToString(mac.Sum(nil))
}

func otpMatches(key []byte, otp, stored string) bool {
	got := hashOTP(key, otp)
	if len(got) != len(stored) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(stored)) == 1
}

func otpKey(role Role, lookup string) string {
	return "otp:" + string(role) + ":" + lookup
}
