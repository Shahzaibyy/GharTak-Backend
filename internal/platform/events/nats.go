package events

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
)

const streamName = "GHARTAK"

type NATS struct {
	conn *nats.Conn
	js   nats.JetStreamContext
}

func OpenNATS(url string) (*NATS, error) {
	conn, err := nats.Connect(url, nats.Timeout(5*time.Second))
	if err != nil {
		return nil, fmt.Errorf("events: nats connect: %w", err)
	}
	js, err := conn.JetStream()
	if err != nil {
		_ = conn.Drain()
		return nil, fmt.Errorf("events: jetstream: %w", err)
	}
	bus := &NATS{conn: conn, js: js}
	if err := bus.ensureStream(); err != nil {
		_ = conn.Drain()
		return nil, err
	}
	return bus, nil
}

func (n *NATS) ensureStream() error {
	if _, err := n.js.StreamInfo(streamName); err == nil {
		return nil
	}
	_, err := n.js.AddStream(&nats.StreamConfig{
		Name:     streamName,
		Subjects: []string{"ghartak.events.>"},
		Storage:  nats.FileStorage,
	})
	if err != nil {
		return fmt.Errorf("events: stream: %w", err)
	}
	return nil
}

func (n *NATS) PublishOrderStatus(ctx context.Context, event OrderStatus) error {
	body, err := event.Marshal()
	if err != nil {
		return err
	}
	_, err = n.js.Publish(SubjectOrderStatus, body, nats.Context(ctx))
	return err
}

func (n *NATS) Close() {
	if n.conn != nil {
		_ = n.conn.Drain()
	}
}

func (n *NATS) SubscribeNotify(ctx context.Context, jobs JobSink, log zerolog.Logger) error {
	_, err := n.js.Subscribe(SubjectOrderStatus, func(msg *nats.Msg) {
		event, err := UnmarshalOrderStatus(msg.Data)
		if err != nil {
			log.Error().Err(err).Msg("nats order status decode")
			_ = msg.Nak()
			return
		}
		if err := jobs.EnqueueOrderStatus(ctx, event); err != nil {
			log.Error().Err(err).Msg("nats enqueue notify")
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	}, nats.Durable("notify-enqueuer"), nats.ManualAck())
	return err
}
