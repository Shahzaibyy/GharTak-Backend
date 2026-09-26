package dispatch

type OfferResponse string

const (
	OfferAccepted OfferResponse = "accepted"
	OfferRejected OfferResponse = "rejected"
	OfferExpired  OfferResponse = "expired"
)

func OfferResponses() []OfferResponse {
	return []OfferResponse{OfferAccepted, OfferRejected, OfferExpired}
}
