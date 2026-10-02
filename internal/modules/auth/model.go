package auth

import (
	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/money"
)

type Account struct {
	ID     uuid.UUID
	Status string
}

type Session struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int       `json:"expires_in"`
	AccountID    uuid.UUID `json:"account_id"`
	Role         Role      `json:"role"`
	Status       string    `json:"status"`
}

type OTPRequest struct {
	Phone   string
	Role    Role
	IP      string
	Channel string
}

type VerifyRequest struct {
	Phone string
	Role  Role
	OTP   string
}

type EmailOTPRequest struct {
	Email string
	IP    string
}

type EmailVerifyRequest struct {
	Email string
	OTP   string
}

type Profile struct {
	ID                   uuid.UUID   `json:"id"`
	Name                 *string     `json:"name"`
	Email                *string     `json:"email"`
	Phone                *string     `json:"phone"`
	PhoneVerified        bool        `json:"phone_verified"`
	WalletBalance        money.Money `json:"wallet_balance"`
	PreferredOrderTypes  []string    `json:"preferred_order_types,omitempty"`
}

type Identity struct {
	UID   string
	Email string
	Name  string
}

type googleInsert struct {
	UID   string
	Email *string
	Name  *string
}

type profileRow struct {
	ID                  uuid.UUID
	Name                *string
	Email               *string
	PhoneCipher         *string
	PhoneVerified       bool
	Wallet              string
	PreferredOrderTypes []string
}

type userLock struct {
	deleted bool
	zero    bool
}
