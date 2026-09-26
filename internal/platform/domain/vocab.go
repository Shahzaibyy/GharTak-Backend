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
