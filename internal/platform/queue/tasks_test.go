package queue

import (
	"testing"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/modules/notifications"
)

func TestNotifyPayloadRoundTrip(t *testing.T) {
	merchant := uuid.MustParse("22222222-2222-4222-8222-222222222201")
	notice := notifications.Notice{
		OrderID: uuid.MustParse("33333333-3333-4333-8333-333333333301"),
		Status:  "placed",
		CustomerID: uuid.MustParse("44444444-4444-4444-8444-444444444401"),
		MerchantID: &merchant,
	}
	body, err := encodeNotify(notice)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeNotify(body)
	if err != nil {
		t.Fatal(err)
	}
	if got.OrderID != notice.OrderID || got.Status != notice.Status || got.MerchantID == nil {
		t.Fatalf("got = %+v", got)
	}
}

func TestOfferPayloadRoundTrip(t *testing.T) {
	id := uuid.MustParse("55555555-5555-4555-8555-555555555501")
	body, err := encodeOffer(id)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeOffer(body)
	if err != nil || got != id {
		t.Fatalf("got = %v err = %v", got, err)
	}
}
