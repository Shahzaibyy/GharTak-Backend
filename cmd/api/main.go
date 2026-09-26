package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/yourusername/ghartak-backend/internal/platform/config"
	"github.com/yourusername/ghartak-backend/internal/platform/database"
	"github.com/yourusername/ghartak-backend/internal/platform/httpserver"
	"github.com/yourusername/ghartak-backend/internal/platform/kv"
	"github.com/yourusername/ghartak-backend/internal/platform/logger"
)

type application struct {
	cfg    config.Config
	log    zerolog.Logger
	pool   *pgxpool.Pool
	redis  *redis.Client
	server *http.Server
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "api: %s\n", err)
		os.Exit(1)
	}
}

func run() error {
	app, err := wire()
	if err != nil {
		return err
	}
	defer app.close()
	return app.serve()
}

func wire() (*application, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return open(context.Background(), cfg, logger.New(cfg.AppEnv))
}

func open(ctx context.Context, cfg config.Config, log zerolog.Logger) (*application, error) {
	app, err := connectApp(ctx, cfg, log)
	if err != nil {
		return nil, err
	}
	handler, err := app.handler()
	if err != nil {
		app.close()
		return nil, err
	}
	app.server = httpserver.New(cfg.HTTPAddr, handler)
	return app, nil
}

func connectApp(ctx context.Context, cfg config.Config, log zerolog.Logger) (*application, error) {
	pool, err := connect(ctx, cfg)
	if err != nil {
		return nil, err
	}
	client, err := kv.New(cfg.RedisURL)
	if err != nil {
		pool.Close()
		return nil, err
	}
	log.Info().Str("env", cfg.AppEnv).Msg("database ready")
	return &application{cfg: cfg, log: log, pool: pool, redis: client}, nil
}

func (a *application) close() {
	a.pool.Close()
	if a.redis != nil {
		_ = a.redis.Close()
	}
}

func connect(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	pool, err := database.NewPool(ctx, database.PoolConfig{URL: cfg.DatabaseURL, MaxConns: cfg.DBMaxConns})
	if err != nil {
		return nil, err
	}
	mctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := database.Migrate(mctx, pool, cfg.MigrationsDir); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func (a *application) serve() error {
	errCh := make(chan error, 1)
	go func() {
		a.log.Info().Str("addr", a.cfg.HTTPAddr).Msg("listening")
		errCh <- a.server.ListenAndServe()
	}()
	return a.wait(errCh)
}

func (a *application) wait(errCh <-chan error) error {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stop)
	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-stop:
		return a.shutdown()
	}
}

func (a *application) shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	a.log.Info().Msg("shutting down")
	return a.server.Shutdown(ctx)
}
