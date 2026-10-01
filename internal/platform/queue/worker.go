package queue

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	"github.com/yourusername/ghartak-backend/internal/modules/notifications"
)

type notifySender interface {
	OrderStatus(ctx context.Context, notice notifications.Notice)
}

type offerer interface {
	Offer(ctx context.Context, orderID uuid.UUID) error
}

type Worker struct {
	server *asynq.Server
	mux    *asynq.ServeMux
}

func NewWorker(url string) (*Worker, error) {
	opt, err := RedisOpt(url)
	if err != nil {
		return nil, err
	}
	srv := asynq.NewServer(opt, asynq.Config{
		Concurrency: 4,
		Queues: map[string]int{
			QueueCritical: 6,
			QueueDefault:  3,
			QueueLow:      1,
		},
	})
	return &Worker{server: srv, mux: asynq.NewServeMux()}, nil
}

func (w *Worker) Handle(notify notifySender, offers offerer, log zerolog.Logger) {
	w.mux.HandleFunc(TaskNotify, handleNotify(notify))
	w.mux.HandleFunc(TaskOfferTimeout, handleOfferTimeout(offers, log))
	w.mux.HandleFunc(TaskFraudScan, handleFraudScan(log))
}

func (w *Worker) Run() error {
	return w.server.Run(w.mux)
}

func (w *Worker) Shutdown() {
	w.server.Shutdown()
}

func handleNotify(sender notifySender) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, task *asynq.Task) error {
		notice, err := decodeNotify(task.Payload())
		if err != nil {
			return fmt.Errorf("queue: notify payload: %w", err)
		}
		sender.OrderStatus(ctx, notice)
		return nil
	}
}

func handleOfferTimeout(offers offerer, log zerolog.Logger) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, task *asynq.Task) error {
		orderID, err := decodeOffer(task.Payload())
		if err != nil {
			return fmt.Errorf("queue: offer payload: %w", err)
		}
		if err := offers.Offer(ctx, orderID); err != nil {
			log.Warn().Err(err).Str("order_id", orderID.String()).Msg("offer timeout re-offer")
			return err
		}
		return nil
	}
}

func handleFraudScan(log zerolog.Logger) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, task *asynq.Task) error {
		log.Info().Msg("fraud scan job ran")
		return nil
	}
}

// OfferTimer schedules delayed re-offers after a rider is assigned an offer row.
type OfferTimer struct {
	client *Client
}

func NewOfferTimer(client *Client) *OfferTimer {
	return &OfferTimer{client: client}
}

func (t *OfferTimer) ScheduleOfferTimeout(ctx context.Context, orderID uuid.UUID) error {
	if t == nil || t.client == nil {
		return nil
	}
	return t.client.ScheduleOfferTimeout(ctx, orderID)
}
