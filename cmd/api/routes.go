package main

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/yourusername/ghartak-backend/internal/modules/admin"
	"github.com/yourusername/ghartak-backend/internal/modules/auth"
	"github.com/yourusername/ghartak-backend/internal/modules/customers"
	"github.com/yourusername/ghartak-backend/internal/modules/merchants"
	"github.com/yourusername/ghartak-backend/internal/modules/orders"
	"github.com/yourusername/ghartak-backend/internal/platform/httpserver"
)

type modules struct {
	log       zerolog.Logger
	pool      *pgxpool.Pool
	jwt       []byte
	auth      *auth.Handler
	zones     *admin.Handler
	merchants *merchants.Handler
	customers *customers.Handler
	orders    *orders.Handler
}

func (a *application) handler() (http.Handler, error) {
	built, err := buildModules(a)
	if err != nil {
		return nil, err
	}
	return routes(built), nil
}

func buildModules(app *application) (modules, error) {
	adminSvc := admin.NewService(admin.NewRepository(app.pool))
	merchantSvc := merchants.NewService(merchants.NewRepository(app.pool), adminSvc, app.cfg.PIIKey, app.cfg.PhoneHashKey)
	if err := maybeSeed(app, merchantSvc); err != nil {
		return modules{}, err
	}
	store := auth.NewRedisStore(app.redis)
	authSvc := auth.NewService(auth.NewRepository(app.pool), store, store, store, app.cfg.PIIKey, app.cfg.PhoneHashKey, app.cfg.JWTKey, app.cfg.AppEnv == "development")
	orderSvc := orders.NewService(adminSvc, merchantSvc, orders.NewRepository(app.pool))
	return modules{
		log: app.log, pool: app.pool, jwt: app.cfg.JWTKey,
		auth:      auth.NewHandler(authSvc, app.log),
		zones:     admin.NewHandler(adminSvc, app.log),
		merchants: merchants.NewHandler(merchantSvc, app.log),
		customers: customers.NewHandler(customers.NewService(customers.NewRepository(app.pool)), app.log),
		orders:    orders.NewHandler(orderSvc, app.log),
	}, nil
}

func maybeSeed(app *application, merchantsSvc *merchants.Service) error {
	if app.cfg.AppEnv != "development" {
		return nil
	}
	return merchantsSvc.SeedDemo(context.Background())
}

func routes(m modules) http.Handler {
	limiter := httpserver.NewLimiter(120, time.Minute)
	router := httpserver.NewMux(m.log)
	router.Group(func(r chi.Router) {
		r.Use(middleware.Timeout(15 * time.Second))
		r.Use(limiter.Middleware)
		publicRoutes(r, m)
		privateRoutes(r, m)
	})
	return router
}

func publicRoutes(r chi.Router, m modules) {
	r.Get("/health", httpserver.Health(m.pool, m.log))
	r.Get("/zones", m.zones.ListZones)
	r.Post("/auth/otp/request", m.auth.RequestOTP)
	r.Post("/auth/otp/verify", m.auth.Verify)
	r.Post("/auth/refresh", m.auth.Refresh)
	r.Post("/merchants/register", m.merchants.Register)
	r.Get("/merchants", m.merchants.List)
	r.Get("/merchants/{id}/catalog", m.merchants.Catalog)
}

func privateRoutes(r chi.Router, m modules) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth(m.jwt))
		customerRoutes(r, m)
		merchantRoutes(r, m)
	})
}

func customerRoutes(r chi.Router, m modules) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireRole(auth.RoleCustomer))
		r.Get("/addresses", m.customers.List)
		r.Post("/addresses", m.customers.Create)
		r.Patch("/addresses/{id}", m.customers.Update)
		r.Delete("/addresses/{id}", m.customers.Delete)
		r.Post("/orders/quote", m.orders.Quote)
		r.Post("/orders", m.orders.Place)
		r.Get("/orders/{id}", m.orders.Get)
	})
}

func merchantRoutes(r chi.Router, m modules) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireRole(auth.RoleMerchant))
		r.Post("/merchants/{id}/catalog", m.merchants.AddItem)
		r.Patch("/merchants/{id}/catalog/{item_id}", m.merchants.UpdateItem)
		r.Delete("/merchants/{id}/catalog/{item_id}", m.merchants.DeleteItem)
	})
}
