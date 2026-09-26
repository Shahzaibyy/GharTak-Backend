package support

import (
	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
)

func allowRate(party orderParty, in RatingInput) error {
	if party.Status != "delivered" {
		return apperror.Conflict("ratings are available after delivery")
	}
	if !partyHas(party, in.RaterRole, in.RaterID) {
		return apperror.Forbidden("rater is not a party to this order")
	}
	return validScore(in)
}

func validScore(in RatingInput) error {
	if in.Score < 1 || in.Score > 5 {
		return apperror.Invalid("score is invalid")
	}
	if len(in.Comment) > 1000 {
		return apperror.Invalid("comment is invalid")
	}
	return nil
}

func partyHas(party orderParty, role string, id uuid.UUID) bool {
	check, ok := partyRoles[role]
	if !ok {
		return false
	}
	return check(party, id)
}

var partyRoles = map[string]func(orderParty, uuid.UUID) bool{
	"customer": func(p orderParty, id uuid.UUID) bool { return p.CustomerID == id },
	"rider":    func(p orderParty, id uuid.UUID) bool { return p.RiderID != nil && *p.RiderID == id },
	"merchant": func(p orderParty, id uuid.UUID) bool { return p.MerchantID != nil && *p.MerchantID == id },
}

func validBody(body string, max int) error {
	if len(body) < 1 || len(body) > max {
		return apperror.Invalid("message is invalid")
	}
	return nil
}
