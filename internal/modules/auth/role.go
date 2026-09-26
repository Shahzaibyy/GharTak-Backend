package auth

type Role string

const (
	RoleCustomer Role = "customer"
	RoleRider    Role = "rider"
	RoleMerchant Role = "merchant"
	RoleAdmin    Role = "admin"
)

func ParseRole(raw string) (Role, bool) {
	_, ok := roles[Role(raw)]
	return Role(raw), ok
}

var roles = map[Role]struct{}{
	RoleCustomer: {},
	RoleRider:    {},
	RoleMerchant: {},
	RoleAdmin:    {},
}
