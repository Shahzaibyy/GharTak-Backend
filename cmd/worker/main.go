package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/yourusername/ghartak-backend/internal/modules/dispatch"
	"github.com/yourusername/ghartak-backend/internal/modules/notifications"
	"github.com/yourusername/ghartak-backend/internal/platform/config"
	"github.com/yourusername/ghartak-backend/internal/platform/database"
	"github.com/yourusername/ghartak-backend/internal/platform/events"
	"github.com/yourusername/ghartak-backend/internal/platform/kv"
	"github.com/yourusername/ghartak-backend/internal/platform/logger"
	"github.com/yourusername/ghartak-backend/internal/platform/queue"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "worker: %s\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logger.New(cfg.AppEnv)
	ctx := context.Background()
	pool, err := database.NewPool(ctx, database.PoolConfig{URL: cfg.DatabaseURL, MaxConns: cfg.DBMaxConns})
	if err != nil {
		return err
	}
	defer pool.Close()
	client, err := queue.NewClient(cfg.QueueRedisURL())
	if err != nil {
		return err
	}
	defer client.Close()
	_, nats, _, err := events.Open(cfg.NATSURL, client)
	if err != nil {
		return err
	}
	if nats != nil {
		defer nats.Close()
		if err := nats.SubscribeNotify(ctx, client, log); err != nil {
			return err
		}
	}
	cache, err := kv.New(cfg.RedisURL)
	if err != nil {
		return err
	}
	defer cache.Close()
	sender := notifications.NewSender(log, cfg.FCMServerKey, notifications.NewRepository(pool))
	geo := dispatch.NewGeo(cache)
	disp := dispatch.NewService(dispatch.NewRepository(pool), geo).UseOfferTimer(queue.NewOfferTimer(client))
	worker, err := queue.NewWorker(cfg.QueueRedisURL())
	if err != nil {
		return err
	}
	worker.Handle(sender, disp, log)
	sched, err := queue.NewScheduler(cfg.QueueRedisURL())
	if err != nil {
		return err
	}
	go func() {
		if err := sched.Run(); err != nil {
			log.Error().Err(err).Msg("scheduler stopped")
		}
	}()
	defer sched.Shutdown()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	errCh := make(chan error, 1)
	go func() { errCh <- worker.Run() }()
	select {
	case err := <-errCh:
		return err
	case <-stop:
		worker.Shutdown()
		return nil
	}
}
