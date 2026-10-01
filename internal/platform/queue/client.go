package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/yourusername/ghartak-backend/internal/modules/notifications"
	"github.com/yourusername/ghartak-backend/internal/platform/events"
)

type Client struct {
	inner *asynq.Client
}

func NewClient(url string) (*Client, error) {
	opt, err := RedisOpt(url)
	if err != nil {
		return nil, err
	}
	return &Client{inner: asynq.NewClient(opt)}, nil
}

func (c *Client) Close() error {
	return c.inner.Close()
}

func (c *Client) EnqueueOrderStatus(ctx context.Context, event events.OrderStatus) error {
	return c.EnqueueNotify(ctx, notifications.Notice{
		OrderID: event.OrderID, Status: event.Status, CustomerID: event.CustomerID,
		MerchantID: event.MerchantID, RiderID: event.RiderID,
	})
}

func (c *Client) EnqueueNotify(ctx context.Context, notice notifications.Notice) error {
	body, err := encodeNotify(notice)
	if err != nil {
		return fmt.Errorf("queue: encode notify: %w", err)
	}
	task := asynq.NewTask(TaskNotify, body,
		asynq.Queue(QueueLow),
		asynq.MaxRetry(notifyMaxRetry),
		asynq.Timeout(notifyTimeout),
		asynq.Unique(10*time.Minute),
	)
	_, err = c.inner.EnqueueContext(ctx, task)
	return err
}

func (c *Client) ScheduleOfferTimeout(ctx context.Context, orderID uuid.UUID) error {
	body, err := encodeOffer(orderID)
	if err != nil {
		return fmt.Errorf("queue: encode offer: %w", err)
	}
	task := asynq.NewTask(TaskOfferTimeout, body,
		asynq.Queue(QueueCritical),
		asynq.ProcessIn(offerTimeoutDelay),
		asynq.TaskID(offerTaskID(orderID)),
		asynq.Unique(offerUniqueTTL),
		asynq.MaxRetry(5),
		asynq.Timeout(20*time.Second),
	)
	_, err = c.inner.EnqueueContext(ctx, task)
	return err
}

func offerTaskID(orderID uuid.UUID) string {
	return "offer-timeout:" + orderID.String()
}
