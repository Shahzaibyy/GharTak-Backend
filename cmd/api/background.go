package main

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/yourusername/ghartak-backend/internal/modules/dispatch"
	"github.com/yourusername/ghartak-backend/internal/modules/notifications"
	"github.com/hibiken/asynq"

	"github.com/yourusername/ghartak-backend/internal/platform/config"
	"github.com/yourusername/ghartak-backend/internal/platform/events"
	"github.com/yourusername/ghartak-backend/internal/platform/queue"
)

type background struct {
	queueClient *queue.Client
	worker      *queue.Worker
	scheduler   *asynq.Scheduler
	nats        *events.NATS
}

func startBackground(ctx context.Context, cfg config.Config, log zerolog.Logger, client *queue.Client, nats *events.NATS, sender *notifications.Sender, disp *dispatch.Service) (*background, error) {
	bg := &background{queueClient: client, nats: nats}
	if nats != nil {
		if err := nats.SubscribeNotify(ctx, client, log); err != nil {
			return nil, err
		}
	}
	if !cfg.RunAsynqWorker {
		return bg, nil
	}
	worker, err := queue.NewWorker(cfg.QueueRedisURL())
	if err != nil {
		bg.close()
		return nil, err
	}
	worker.Handle(sender, disp, log)
	bg.worker = worker
	sched, err := queue.NewScheduler(cfg.QueueRedisURL())
	if err != nil {
		bg.close()
		return nil, err
	}
	bg.scheduler = sched
	go func() {
		if err := worker.Run(); err != nil {
			log.Error().Err(err).Msg("asynq worker stopped")
		}
	}()
	go func() {
		if err := sched.Run(); err != nil {
			log.Error().Err(err).Msg("asynq scheduler stopped")
		}
	}()
	log.Info().Msg("asynq worker and scheduler running")
	return bg, nil
}

func (b *background) close() {
	if b.scheduler != nil {
		b.scheduler.Shutdown()
	}
	if b.worker != nil {
		b.worker.Shutdown()
	}
	if b.nats != nil {
		b.nats.Close()
	}
	if b.queueClient != nil {
		_ = b.queueClient.Close()
	}
}
