package merchants

type Category string

const (
	CategoryRestaurant Category = "restaurant"
	CategoryMart       Category = "mart"
	CategoryPharmacy   Category = "pharmacy"
)

func Categories() []Category {
	return []Category{CategoryRestaurant, CategoryMart, CategoryPharmacy}
}

// DefaultCommissionPercent is the rate stored on a new merchant for that category.
func DefaultCommissionPercent(category Category) (string, bool) {
	rate, ok := defaultCommission[category]
	return rate, ok
}

var defaultCommission = map[Category]string{
	CategoryRestaurant: "18.00",
	CategoryMart:       "10.00",
	CategoryPharmacy:   "10.00",
}
