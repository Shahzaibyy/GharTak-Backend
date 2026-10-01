package queue

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/modules/notifications"
)

const (
	TaskNotify        = "notify:order_status"
	TaskOfferTimeout  = "dispatch:offer_timeout"
	TaskFraudScan     = "admin:fraud_scan"
	QueueCritical     = "critical"
	QueueDefault      = "default"
	QueueLow          = "low"
	offerTimeoutDelay = 30 * time.Second
	offerUniqueTTL    = time.Hour
	notifyTimeout     = 30 * time.Second
	notifyMaxRetry    = 8
)

type notifyPayload struct {
	OrderID    uuid.UUID  `json:"order_id"`
	Status     string     `json:"status"`
	CustomerID uuid.UUID  `json:"customer_id"`
	MerchantID *uuid.UUID `json:"merchant_id,omitempty"`
	RiderID    *uuid.UUID `json:"rider_id,omitempty"`
}

type offerPayload struct {
	OrderID uuid.UUID `json:"order_id"`
}

func noticeFrom(raw notifyPayload) notifications.Notice {
	return notifications.Notice{
		OrderID: raw.OrderID, Status: raw.Status, CustomerID: raw.CustomerID,
		MerchantID: raw.MerchantID, RiderID: raw.RiderID,
	}
}

func payloadFrom(notice notifications.Notice) notifyPayload {
	return notifyPayload{
		OrderID: notice.OrderID, Status: notice.Status, CustomerID: notice.CustomerID,
		MerchantID: notice.MerchantID, RiderID: notice.RiderID,
	}
}

func encodeNotify(notice notifications.Notice) ([]byte, error) {
	return json.Marshal(payloadFrom(notice))
}

func decodeNotify(body []byte) (notifications.Notice, error) {
	var raw notifyPayload
	if err := json.Unmarshal(body, &raw); err != nil {
		return notifications.Notice{}, err
	}
	return noticeFrom(raw), nil
}

func encodeOffer(orderID uuid.UUID) ([]byte, error) {
	return json.Marshal(offerPayload{OrderID: orderID})
}

func decodeOffer(body []byte) (uuid.UUID, error) {
	var raw offerPayload
	if err := json.Unmarshal(body, &raw); err != nil {
		return uuid.UUID{}, err
	}
	return raw.OrderID, nil
}
