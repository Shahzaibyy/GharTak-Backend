package notifications

import (
	"context"

	"github.com/yourusername/ghartak-backend/internal/platform/events"
)

type eventPublisher interface {
	PublishOrderStatus(ctx context.Context, event events.OrderStatus) error
}

type EventNotifier struct {
	bus eventPublisher
}

func NewEventNotifier(bus eventPublisher) *EventNotifier {
	return &EventNotifier{bus: bus}
}

func (n *EventNotifier) OrderStatus(ctx context.Context, notice Notice) {
	if n == nil || n.bus == nil {
		return
	}
	_ = n.bus.PublishOrderStatus(ctx, events.OrderStatus{
		OrderID: notice.OrderID, Status: notice.Status, CustomerID: notice.CustomerID,
		MerchantID: notice.MerchantID, RiderID: notice.RiderID,
	})
}
