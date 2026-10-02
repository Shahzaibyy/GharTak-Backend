package main

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/yourusername/ghartak-backend/internal/modules/admin"
	"github.com/yourusername/ghartak-backend/internal/modules/auth"
	"github.com/yourusername/ghartak-backend/internal/modules/chat"
	"github.com/yourusername/ghartak-backend/internal/modules/customers"
	"github.com/yourusername/ghartak-backend/internal/modules/dispatch"
	"github.com/yourusername/ghartak-backend/internal/modules/geo"
	"github.com/yourusername/ghartak-backend/internal/modules/merchants"
	"github.com/yourusername/ghartak-backend/internal/modules/notifications"
	"github.com/yourusername/ghartak-backend/internal/modules/orders"
	"github.com/yourusername/ghartak-backend/internal/modules/payments"
	"github.com/yourusername/ghartak-backend/internal/modules/realtime"
	"github.com/yourusername/ghartak-backend/internal/modules/riders"
	"github.com/yourusername/ghartak-backend/internal/modules/support"
	"github.com/yourusername/ghartak-backend/internal/platform/events"
	"github.com/yourusername/ghartak-backend/internal/platform/httpserver"
	"github.com/yourusername/ghartak-backend/internal/platform/mapbox"
	"github.com/yourusername/ghartak-backend/internal/platform/notify"
	"github.com/yourusername/ghartak-backend/internal/platform/queue"
	"github.com/yourusername/ghartak-backend/internal/platform/uploads"
)

type modules struct {
	log          zerolog.Logger
	pool         *pgxpool.Pool
	jwt          []byte
	auth         *auth.Handler
	zones        *admin.Handler
	merchants    *merchants.Handler
	customers    *customers.Handler
	orders       *orders.Handler
	riders       *riders.Handler
	payments     *payments.Handler
	support      *support.Handler
	chat         *chat.Handler
	notify       *notifications.Handler
	realtime     *realtime.Handler
	uploads      *uploads.Handler
	places       *geo.Handler
	queueMonitor http.Handler
}

func (a *application) handler() (http.Handler, error) {
	built, bg, err := buildModules(a)
	if err != nil {
		return nil, err
	}
	a.bg = bg
	return routes(built), nil
}

func buildModules(app *application) (modules, *background, error) {
	adminRepo := admin.NewRepository(app.pool)
	adminSvc := admin.NewService(adminRepo)
	merchantSvc := merchants.NewService(merchants.NewRepository(app.pool), adminSvc, app.cfg.PIIKey, app.cfg.PhoneHashKey)
	queueClient, err := queue.NewClient(app.cfg.QueueRedisURL())
	if err != nil {
		return modules{}, nil, err
	}
	bus, nats, _, err := events.Open(app.cfg.NATSURL, queueClient)
	if err != nil {
		queueClient.Close()
		return modules{}, nil, err
	}
	store := auth.NewRedisStore(app.redis)
	authSvc := auth.NewService(auth.NewRepository(app.pool), store, store, store, app.cfg.PIIKey, app.cfg.PhoneHashKey, app.cfg.JWTKey, app.cfg.AppEnv == "development")
	identity, err := auth.OpenIdentity(context.Background(), app.cfg.FirebaseProjectID, app.cfg.FirebaseCredentialsFile, app.cfg.FirebaseCredentialsJSON)
	if err != nil {
		queueClient.Close()
		return modules{}, nil, err
	}
	authSvc.UseIdentity(identity)
	authSvc.UseSender(notify.NewLogSender(app.log))
	payRepo := payments.NewRepository(app.pool)
	noteRepo := notifications.NewRepository(app.pool)
	sender := notifications.NewSender(app.log, app.cfg.FCMServerKey, noteRepo)
	notifier := notifications.NewEventNotifier(bus)
	mapClient := mapbox.NewClient(mapbox.Settings{
		Token: app.cfg.MapboxToken, BaseURL: app.cfg.MapboxBaseURL, Timeout: app.cfg.MapboxTimeout,
	})
	routeCache := mapbox.NewRedisCache(app.redis)
	router := mapbox.NewFallbackRouter(
		mapbox.NewCachedRouter(mapClient, routeCache, app.cfg.MapboxRouteCacheTTL),
		app.cfg.RouteCircuityFactor,
		app.cfg.RouteFallbackKMH,
	)
	geocoder := mapbox.NewCachedGeocoder(mapClient, routeCache, app.cfg.MapboxGeoSearchTTL, app.cfg.MapboxGeoReverseTTL)
	riderGeo := dispatch.NewGeo(app.redis)
	dispSvc := dispatch.NewService(dispatch.NewRepository(app.pool), riderGeo).UseOfferTimer(queue.NewOfferTimer(queueClient))
	orderRepo := orders.NewRepository(app.pool).WithBooks(payRepo)
	orderSvc := orders.NewService(adminSvc, merchantSvc, orderRepo).WithFlow(app.cfg.PhoneHashKey, notifier, dispSvc).UsePhoneGate(authSvc).UseRouter(router)
	riderSvc := riders.NewService(riders.NewRepository(app.pool), adminSvc, app.cfg.PIIKey, app.cfg.PhoneHashKey)
	riderSvc.UsePresence(riderGeo)
	riderSvc.UseApply(riders.NewRedisApply(app.redis))
	riderSvc.UseOTP(authSvc)
	authSvc.UseRiderBootstrap(riderSvc)
	customerSvc := customers.NewService(customers.NewRepository(app.pool))
	customerSvc.UseProfiles(authSvc)
	if err := maybeSeed(app, adminRepo, merchantSvc, riderSvc, authSvc); err != nil {
		queueClient.Close()
		return modules{}, nil, err
	}
	chatSvc := chat.NewService(chat.NewRepository(app.pool), orderSvc, app.redis, app.log)
	supportSvc := support.NewService(support.NewRepository(app.pool))
	placeSvc := geo.NewService(geocoder, func(ctx context.Context, id uuid.UUID) (float64, float64, error) {
		zone, err := adminSvc.ActiveZone(ctx, id)
		if err != nil {
			return 0, 0, err
		}
		return zone.CenterLat, zone.CenterLng, nil
	}, geo.NewRedisLimiter(app.redis))
	realtimeHandler := realtime.NewHandler(app.cfg.JWTKey, app.redis, orderSvc, chatSvc, app.log)
	realtimeHandler.UseETA(router)
	monitor, err := queue.MonitorHandler(app.cfg.QueueRedisURL())
	if err != nil {
		queueClient.Close()
		return modules{}, nil, err
	}
	bg, err := startBackground(context.Background(), app.cfg, app.log, queueClient, nats, sender, dispSvc)
	if err != nil {
		queueClient.Close()
		return modules{}, nil, err
	}
	return modules{
		log: app.log, pool: app.pool, jwt: app.cfg.JWTKey,
		auth:      auth.NewHandler(authSvc, app.log),
		zones:     admin.NewHandler(adminSvc, app.log),
		merchants: merchants.NewHandler(merchantSvc, app.log),
		customers: customers.NewHandler(customerSvc, app.log),
		orders:    orders.NewHandler(orderSvc, app.log),
		riders:    riders.NewHandler(riderSvc, app.log),
		payments:  payments.NewHandler(payments.NewService(payRepo), app.log),
		support:   support.NewHandler(supportSvc, app.log),
		chat:      chat.NewHandler(chatSvc, app.log),
		notify:    notifications.NewHandler(sender, app.log),
		realtime:  realtimeHandler,
		places:    geo.NewHandler(placeSvc, app.log),
		uploads: uploads.NewHandler(uploads.NewSigner(uploads.Settings{
			Endpoint: app.cfg.S3Endpoint, Bucket: app.cfg.S3Bucket, Region: app.cfg.S3Region,
			AccessKey: app.cfg.S3AccessKey, SecretKey: app.cfg.S3SecretKey,
		}), auth.AccountID, app.log),
		queueMonitor: monitor,
	}, bg, nil
}

func maybeSeed(
	app *application,
	adminRepo *admin.Repository,
	merchantsSvc *merchants.Service,
	riderSvc *riders.Service,
	authSvc *auth.Service,
) error {
	if app.cfg.AppEnv != "development" {
		return nil
	}
	ctx := context.Background()
	if err := merchantsSvc.SeedDemo(ctx); err != nil {
		return err
	}
	if err := merchantsSvc.SeedFatehJang(ctx); err != nil {
		return err
	}
	if err := adminRepo.SeedDemoAdmin(ctx, app.cfg.PIIKey, app.cfg.PhoneHashKey); err != nil {
		return err
	}
	if err := authSvc.SeedFatehJangCustomers(ctx, app.pool); err != nil {
		return err
	}
	return riderSvc.SeedFatehJang(ctx, app.pool)
}

func routes(m modules) http.Handler {
	limiter := httpserver.NewLimiter(120, time.Minute)
	router := httpserver.NewMux(m.log)
	mountSockets(router, m)
	router.Group(func(r chi.Router) {
		r.Use(middleware.Timeout(15 * time.Second))
		r.Use(limiter.Middleware)
		mountDocs(r)
		publicRoutes(r, m)
		privateRoutes(r, m)
	})
	return router
}

func mountSockets(r chi.Router, m modules) {
	r.Get("/ws/location/{order_id}", m.realtime.Location)
	r.Get("/ws/chat/{order_id}", m.realtime.Chat)
}

func publicRoutes(r chi.Router, m modules) {
	r.Get("/health", httpserver.Health(m.pool, m.log))
	r.Get("/zones", m.zones.ListZones)
	r.Post("/auth/otp/request", m.auth.RequestOTP)
	r.Post("/auth/otp/verify", m.auth.Verify)
	r.Post("/auth/email/request", m.auth.RequestEmail)
	r.Post("/auth/email/verify", m.auth.VerifyEmail)
	r.Post("/auth/refresh", m.auth.Refresh)
	r.Post("/auth/google", m.auth.Google)
	r.Post("/auth/logout", m.auth.Logout)
	r.Post("/merchants/register", m.merchants.Register)
	r.Get("/merchants", m.merchants.List)
	r.Get("/merchants/{id}/catalog", m.merchants.Catalog)
	r.Post("/riders/register", m.riders.Register)
	r.Post("/riders/onboarding/apply", m.riders.Apply)
	r.Post("/payments/webhooks/{provider}", m.payments.Webhook)
}

func privateRoutes(r chi.Router, m modules) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth(m.jwt))
		sharedRoutes(r, m)
		customerRoutes(r, m)
		merchantRoutes(r, m)
		riderRoutes(r, m)
		adminRoutes(r, m)
	})
}

func customerRoutes(r chi.Router, m modules) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireRole(auth.RoleCustomer))
		r.Get("/users/me", m.auth.Me)
		r.Patch("/users/me", m.auth.UpdateMe)
		r.Delete("/users/me", m.auth.DeleteMe)
		r.Post("/auth/phone/link", m.auth.PhoneLink)
		r.Post("/auth/phone/link/verify", m.auth.PhoneLinkVerify)
		r.Post("/customers/onboarding", m.customers.Onboarding)
		r.Patch("/customers/me/preferences", m.auth.Preferences)
		r.Get("/addresses", m.customers.List)
		r.Post("/addresses", m.customers.Create)
		r.Patch("/addresses/{id}", m.customers.Update)
		r.Delete("/addresses/{id}", m.customers.Delete)
		r.Post("/orders/quote", m.orders.Quote)
		r.Post("/orders", m.orders.Place)
		r.Get("/orders/{id}", m.orders.Get)
		r.Post("/orders/{id}/cancel", m.orders.Cancel)
		r.Post("/tickets", m.support.Open)
		r.Get("/tickets", m.support.List)
		r.Get("/tickets/{id}", m.support.Get)
		r.Get("/tickets/{id}/messages", m.support.Messages)
		r.Post("/tickets/{id}/messages", m.support.Reply)
	})
}

func sharedRoutes(r chi.Router, m modules) {
	r.Get("/geo/search", m.places.Search)
	r.Get("/geo/reverse", m.places.Reverse)
	r.Post("/uploads/presign", m.uploads.Presign)
	r.Post("/notifications/devices", m.notify.Register)
	r.Get("/orders/{id}/messages", m.chat.List)
	r.Post("/orders/{id}/messages", m.chat.Send)
	r.Post("/orders/{id}/dispatch", m.orders.Dispatch)
	r.Post("/orders/{id}/ratings", m.support.Rate)
}

func riderRoutes(r chi.Router, m modules) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireRole(auth.RoleRider))
		r.Get("/riders/me", m.riders.Me)
		r.Get("/riders/me/onboarding", m.riders.OnboardingStatus)
		r.Patch("/riders/me/onboarding/details", m.riders.Details)
		r.Put("/riders/me/onboarding/documents", m.riders.Documents)
		r.Post("/riders/me/onboarding/submit", m.riders.Submit)
		r.Post("/riders/me/onboarding/orientation", m.riders.Orientation)
		r.Post("/riders/availability", m.riders.Availability)
		r.Post("/riders/position", m.riders.Position)
		r.Get("/riders/offers", m.orders.Offers)
		r.Get("/riders/tasks", m.orders.Tasks)
		r.Post("/riders/tasks/{id}/accept", m.orders.Accept)
		r.Post("/riders/tasks/{id}/reject", m.orders.Reject)
		r.Post("/riders/tasks/{id}/pickup", m.orders.Pickup)
		r.Post("/riders/tasks/{id}/enroute", m.orders.Enroute)
		r.Post("/riders/tasks/{id}/deliver", m.orders.Deliver)
	})
}

func adminRoutes(r chi.Router, m modules) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireRole(auth.RoleAdmin))
		if m.queueMonitor != nil {
			r.Handle("/admin/queue/*", m.queueMonitor)
		}
		r.Get("/admin/riders", m.riders.Queue)
		r.Post("/admin/riders/{id}/approve", m.riders.Approve)
		r.Post("/admin/riders/{id}/reject", m.riders.Reject)
		r.Post("/admin/riders/{id}/suspend", m.riders.Suspend)
		r.Post("/admin/merchants/{id}/approve", m.merchants.Approve)
		r.Post("/admin/merchants/{id}/reject", m.merchants.Reject)
		r.Post("/admin/riders/{id}/settle", m.payments.SettleCash)
		r.Post("/admin/wallet/credits", m.payments.CreditWallet)
	})
}

func merchantRoutes(r chi.Router, m modules) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireRole(auth.RoleMerchant))
		r.Post("/merchants/{id}/catalog", m.merchants.AddItem)
		r.Patch("/merchants/{id}/catalog/{item_id}", m.merchants.UpdateItem)
		r.Delete("/merchants/{id}/catalog/{item_id}", m.merchants.DeleteItem)
		r.Get("/merchants/orders", m.orders.MerchantOrders)
		r.Post("/merchants/orders/{id}/{action}", m.orders.MerchantAction)
	})
}
