package riders

import (
	"time"

	"github.com/google/uuid"
)

type Profile struct {
	ID                 uuid.UUID  `json:"id"`
	Name               string     `json:"name"`
	VerificationStatus string     `json:"verification_status"`
	ZoneID             *uuid.UUID `json:"zone_id"`
	VehicleReg         string     `json:"vehicle_reg"`
	IsOnline           bool       `json:"is_online"`
	Rating             string     `json:"rating"`
	RatingCount        int        `json:"rating_count"`
	CNICObjectKey      *string    `json:"cnic_object_key"`
	SelfieObjectKey    *string    `json:"selfie_object_key"`
	LicenseObjectKey   *string    `json:"license_object_key"`
	CreatedAt          time.Time  `json:"created_at"`
}

type RegisterInput struct {
	Phone            string
	Name             string
	CNIC             string
	CNICObjectKey    string
	SelfieObjectKey  string
	LicenseObjectKey string
	VehicleReg       string
	ZoneID           uuid.UUID
}
