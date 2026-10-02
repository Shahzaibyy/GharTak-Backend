package auth

import (
	"context"
	"errors"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
)

// DemoAccount is a one-tap login target for development testing only.
type DemoAccount struct {
	Label string `json:"label"`
	Phone string `json:"phone"`
	Role  Role   `json:"role"`
	Name  string `json:"name,omitempty"`
	Note  string `json:"note,omitempty"`
}

// DemoAccounts returns the seeded Fateh Jang / Attock accounts for the demo portal.
func DemoAccounts() []DemoAccount {
	return []DemoAccount{
		{Label: "Customer · Ayesha", Phone: "03001111001", Role: RoleCustomer, Name: "Ayesha Khan", Note: "Fateh Jang home pin"},
		{Label: "Customer · Ali", Phone: "03001111002", Role: RoleCustomer, Name: "Ali Hassan"},
		{Label: "Customer · Sana", Phone: "03001111003", Role: RoleCustomer, Name: "Sana Malik"},
		{Label: "Customer · Omar", Phone: "03001111004", Role: RoleCustomer, Name: "Omar Farooq"},
		{Label: "Customer · Hira", Phone: "03001111005", Role: RoleCustomer, Name: "Hira Bibi"},
		{Label: "Rider · Usman", Phone: "03002222001", Role: RoleRider, Name: "Usman Ali", Note: "Online near Spanish Pizza"},
		{Label: "Rider · Bilal", Phone: "03002222002", Role: RoleRider, Name: "Bilal Khan"},
		{Label: "Rider · Hamza", Phone: "03002222003", Role: RoleRider, Name: "Hamza Iqbal"},
		{Label: "Rider · Saad", Phone: "03002222004", Role: RoleRider, Name: "Saad Raza"},
		{Label: "Rider · Farhan", Phone: "03002222005", Role: RoleRider, Name: "Farhan Malik"},
		{Label: "Merchant · Bismillah", Phone: "03003333001", Role: RoleMerchant, Name: "Bismillah Restaurant"},
		{Label: "Merchant · Spanish Pizza", Phone: "03003333003", Role: RoleMerchant, Name: "Spanish Pizza Fateh Jang"},
		{Label: "Merchant · Attock demo", Phone: "03000000001", Role: RoleMerchant, Name: "GharTak Demo Kitchen"},
		{Label: "Admin", Phone: "03000000002", Role: RoleAdmin, Name: "GharTak Platform"},
	}
}

// DemoLogin issues a session without OTP. Development only.
func (s *Service) DemoLogin(ctx context.Context, phone string, role Role) (Session, error) {
	if !s.dev {
		return Session{}, apperror.ErrNotFound
	}
	sealed, err := s.seal(phone)
	if err != nil {
		return Session{}, err
	}
	account, err := s.accountForDemo(ctx, role, sealed)
	if err != nil {
		return Session{}, err
	}
	if err := allowStatus(account.Status); err != nil {
		return Session{}, err
	}
	return s.issue(ctx, account, role, "demo-"+account.ID.String())
}

func (s *Service) accountForDemo(ctx context.Context, role Role, sealed sealedPhone) (Account, error) {
	if role == RoleCustomer {
		return s.accounts.UpsertCustomer(ctx, sealed.ciphertext, sealed.lookup)
	}
	account, err := s.accounts.Find(ctx, role, sealed.lookup)
	if errors.Is(err, apperror.ErrNotFound) {
		return Account{}, apperror.Unauthorized("demo account is not registered — restart API with APP_ENV=development so seeds run")
	}
	return account, err
}

// ListDemoAccounts is development-only; production returns not found.
func (s *Service) ListDemoAccounts() ([]DemoAccount, error) {
	if !s.dev {
		return nil, apperror.ErrNotFound
	}
	return DemoAccounts(), nil
}
