package domain

type AccountStatus string

const (
	AccountActive    AccountStatus = "active"
	AccountSuspended AccountStatus = "suspended"
)

func AccountStatuses() []AccountStatus {
	return []AccountStatus{AccountActive, AccountSuspended}
}

type VerificationStatus string

const (
	VerificationPending   VerificationStatus = "pending"
	VerificationApproved  VerificationStatus = "approved"
	VerificationRejected  VerificationStatus = "rejected"
	VerificationSuspended VerificationStatus = "suspended"
)

func VerificationStatuses() []VerificationStatus {
	return []VerificationStatus{
		VerificationPending,
		VerificationApproved,
		VerificationRejected,
		VerificationSuspended,
	}
}

type FraudStatus string

const (
	FraudOpen      FraudStatus = "open"
	FraudReviewed  FraudStatus = "reviewed"
	FraudDismissed FraudStatus = "dismissed"
)

func FraudStatuses() []FraudStatus {
	return []FraudStatus{FraudOpen, FraudReviewed, FraudDismissed}
}

type ActorRole string

const (
	RoleCustomer ActorRole = "customer"
	RoleRider    ActorRole = "rider"
	RoleMerchant ActorRole = "merchant"
	RoleAdmin    ActorRole = "admin"
	RoleSystem   ActorRole = "system"
)

func EventActorRoles() []ActorRole {
	return []ActorRole{RoleCustomer, RoleRider, RoleMerchant, RoleAdmin, RoleSystem}
}

func RatingRoles() []ActorRole {
	return []ActorRole{RoleCustomer, RoleRider, RoleMerchant}
}

type VehicleType string

const (
	VehicleMotorcycle VehicleType = "motorcycle"
	VehicleBicycle    VehicleType = "bicycle"
)

func VehicleTypes() []VehicleType {
	return []VehicleType{VehicleMotorcycle, VehicleBicycle}
}

type OrientationStatus string

const (
	OrientationNone      OrientationStatus = "none"
	OrientationBooked    OrientationStatus = "booked"
	OrientationCompleted OrientationStatus = "completed"
)

func OrientationStatuses() []OrientationStatus {
	return []OrientationStatus{OrientationNone, OrientationBooked, OrientationCompleted}
}

type RiderOnboardingStep string

const (
	OnboardingApplied    RiderOnboardingStep = "applied"
	OnboardingDetails    RiderOnboardingStep = "details"
	OnboardingDocuments  RiderOnboardingStep = "documents"
	OnboardingSubmitted  RiderOnboardingStep = "submitted"
)

func RiderOnboardingSteps() []RiderOnboardingStep {
	return []RiderOnboardingStep{
		OnboardingApplied, OnboardingDetails, OnboardingDocuments, OnboardingSubmitted,
	}
}

type AddressLabel string

const (
	AddressHome  AddressLabel = "home"
	AddressWork  AddressLabel = "work"
	AddressOther AddressLabel = "other"
)

func AddressLabels() []AddressLabel {
	return []AddressLabel{AddressHome, AddressWork, AddressOther}
}
