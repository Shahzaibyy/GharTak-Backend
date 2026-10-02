package riders

import (
	"time"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/domain"
)

type Profile struct {
	ID                   uuid.UUID                   `json:"id"`
	Name                 string                      `json:"name"`
	VerificationStatus   string                      `json:"verification_status"`
	ZoneID               *uuid.UUID                  `json:"zone_id"`
	VehicleReg           string                      `json:"vehicle_reg"`
	VehicleType          *string                     `json:"vehicle_type,omitempty"`
	LicenseNumber        *string                     `json:"license_number,omitempty"`
	IsOnline             bool                        `json:"is_online"`
	Rating               string                      `json:"rating"`
	RatingCount          int                         `json:"rating_count"`
	CNICObjectKey        *string                     `json:"cnic_object_key,omitempty"`
	CNICFrontObjectKey   *string                     `json:"cnic_front_object_key,omitempty"`
	CNICBackObjectKey    *string                     `json:"cnic_back_object_key,omitempty"`
	SelfieObjectKey      *string                     `json:"selfie_object_key,omitempty"`
	LicenseObjectKey     *string                     `json:"license_object_key,omitempty"`
	OrientationStatus    string                      `json:"orientation_status"`
	OrientationSlot      *string                     `json:"orientation_preferred_slot,omitempty"`
	ApplicationReceived  *time.Time                  `json:"application_received_at,omitempty"`
	OnboardingStep       string                      `json:"onboarding_step"`
	CreatedAt            time.Time                   `json:"created_at"`
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

type ApplyInput struct {
	Phone   string
	ZoneID  uuid.UUID
	Channel string
	IP      string
}

type DetailsInput struct {
	Name          string
	CNIC          string
	VehicleType   domain.VehicleType
	VehicleReg    string
	LicenseNumber string
}

type DocumentsInput struct {
	CNICFrontObjectKey string
	CNICBackObjectKey  string
	LicenseObjectKey   string
	SelfieObjectKey    string
}

type OrientationInput struct {
	PreferredSlot string
}

type TimelineStep struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Detail      string `json:"detail"`
	Status      string `json:"status"` // done | current | pending
}

type OnboardingStatus struct {
	Profile  Profile        `json:"profile"`
	Timeline []TimelineStep `json:"timeline"`
}
