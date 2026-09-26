package orders

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
)

func deliveryCode(key []byte) (string, string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return "", "", fmt.Errorf("orders: delivery otp: %w", err)
	}
	otp := fmt.Sprintf("%04d", n.Int64())
	return otp, hashDelivery(key, otp), nil
}

func hashDelivery(key []byte, otp string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("delivery:" + otp))
	return hex.EncodeToString(mac.Sum(nil))
}

func deliveryMatches(key []byte, otp, hash string) bool {
	if hash == "" || len(otp) != 4 {
		return false
	}
	return hmac.Equal([]byte(hash), []byte(hashDelivery(key, otp)))
}

func otpOK(hash, otp string, key []byte) error {
	if len(otp) != 4 {
		return apperror.Invalid("delivery_otp is invalid")
	}
	if !deliveryMatches(key, otp, hash) {
		return apperror.Forbidden("delivery otp does not match")
	}
	return nil
}

func proofOK(order Order, proof string) error {
	if RequiresMerchant(order.Type) {
		return nil
	}
	if len(proof) < 1 || len(proof) > 200 {
		return apperror.Invalid("proof_photo_key is invalid")
	}
	return nil
}
