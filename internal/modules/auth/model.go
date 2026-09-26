package auth

import "github.com/google/uuid"

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
	Phone string
	Role  Role
	IP    string
}

type VerifyRequest struct {
	Phone string
	Role  Role
	OTP   string
}
