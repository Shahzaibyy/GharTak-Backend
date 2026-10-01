package events

import (
	"encoding/json"

	"github.com/google/uuid"
)

const SubjectOrderStatus = "ghartak.events.order.status"

type OrderStatus struct {
	OrderID    uuid.UUID  `json:"order_id"`
	Status     string     `json:"status"`
	CustomerID uuid.UUID  `json:"customer_id"`
	MerchantID *uuid.UUID `json:"merchant_id,omitempty"`
	RiderID    *uuid.UUID `json:"rider_id,omitempty"`
}

func (e OrderStatus) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

func UnmarshalOrderStatus(body []byte) (OrderStatus, error) {
	var event OrderStatus
	if err := json.Unmarshal(body, &event); err != nil {
		return OrderStatus{}, err
	}
	return event, nil
}
