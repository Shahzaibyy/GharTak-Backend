package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/yourusername/ghartak-backend/internal/modules/auth"
	"github.com/yourusername/ghartak-backend/internal/modules/chat"
	"github.com/yourusername/ghartak-backend/internal/modules/orders"
	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/httpx"
)

type parties interface {
	Party(ctx context.Context, orderID uuid.UUID) (orders.Party, error)
}

type chatSender interface {
	Send(ctx context.Context, role string, senderID, orderID uuid.UUID, body string) (chat.Message, error)
}

type Handler struct {
	key     []byte
	redis   *redis.Client
	parties parties
	chat    chatSender
	log     zerolog.Logger
	eta     etaRouter
}

func NewHandler(key []byte, redis *redis.Client, parties parties, chat chatSender, log zerolog.Logger) *Handler {
	return &Handler{key: key, redis: redis, parties: parties, chat: chat, log: log}
}

type session struct {
	principal auth.Principal
	party     orders.Party
}

func (h *Handler) Location(w http.ResponseWriter, r *http.Request) {
	session, err := h.session(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.serveLocation(w, r, session)
}

func (h *Handler) Chat(w http.ResponseWriter, r *http.Request) {
	session, err := h.session(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.serveChat(w, r, session)
}

func (h *Handler) session(r *http.Request) (session, error) {
	principal, orderID, err := socketAuth(r, h.key)
	if err != nil {
		return session{}, err
	}
	party, err := h.parties.Party(r.Context(), orderID)
	if err != nil {
		return session{}, err
	}
	if !party.Allows(string(principal.Role), principal.AccountID) {
		return session{}, apperror.ErrForbidden
	}
	return session{principal: principal, party: party}, nil
}

func (h *Handler) serveLocation(w http.ResponseWriter, r *http.Request, session session) {
	conn, err := accept(w, r)
	if err != nil {
		h.log.Error().Err(err).Msg("location socket")
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.readLocation(ctx, cancel, conn, session)
	h.writeLastPosition(ctx, session.party.OrderID, func(ctx context.Context, raw []byte) error {
		return conn.Write(ctx, websocket.MessageText, raw)
	})
	_ = forward(ctx, conn, h.redis, locationChannel(session.party.OrderID))
}

func (h *Handler) serveChat(w http.ResponseWriter, r *http.Request, session session) {
	conn, err := accept(w, r)
	if err != nil {
		h.log.Error().Err(err).Msg("chat socket")
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.readChat(ctx, cancel, conn, session)
	_ = forward(ctx, conn, h.redis, chat.Channel(session.party.OrderID))
}

func (h *Handler) readLocation(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, session session) {
	defer cancel()
	if session.principal.Role != auth.RoleRider {
		discard(ctx, conn)
		return
	}
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		h.publishLocation(ctx, session.party.OrderID, data, session)
	}
}

func (h *Handler) readChat(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, session session) {
	defer cancel()
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		h.saveChat(ctx, session, data)
	}
}

func (h *Handler) saveChat(ctx context.Context, session session, data []byte) {
	body, ok := chatBody(data)
	if !ok {
		return
	}
	if _, err := h.chat.Send(ctx, string(session.principal.Role), session.principal.AccountID, session.party.OrderID, body); err != nil {
		h.log.Error().Err(err).Str("order_id", session.party.OrderID.String()).Msg("chat socket")
	}
}

func socketAuth(r *http.Request, key []byte) (auth.Principal, uuid.UUID, error) {
	principal, err := auth.ParseAccess(key, r.Header.Get("Authorization"))
	if err != nil {
		return auth.Principal{}, uuid.UUID{}, err
	}
	orderID, err := httpx.ParseUUID(chi.URLParam(r, "order_id"), "order_id")
	if err != nil {
		return auth.Principal{}, uuid.UUID{}, err
	}
	return principal, orderID, nil
}

func accept(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	controller := http.NewResponseController(w)
	_ = controller.SetWriteDeadline(time.Time{})
	_ = controller.SetReadDeadline(time.Time{})
	// The bearer token is checked before upgrade. Browser WebSocket clients cannot set that header.
	return websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
}

func forward(ctx context.Context, conn *websocket.Conn, client *redis.Client, channel string) error {
	sub := client.Subscribe(ctx, channel)
	defer sub.Close()
	messages := sub.Channel()
	for {
		if err := writeNext(ctx, conn, messages); err != nil {
			return done(err)
		}
	}
}

func writeNext(ctx context.Context, conn *websocket.Conn, messages <-chan *redis.Message) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case msg, ok := <-messages:
		return writeMessage(ctx, conn, msg, ok)
	}
}

func writeMessage(ctx context.Context, conn *websocket.Conn, msg *redis.Message, ok bool) error {
	if !ok || msg == nil {
		return context.Canceled
	}
	return conn.Write(ctx, websocket.MessageText, []byte(msg.Payload))
}

func done(err error) error {
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func discard(ctx context.Context, conn *websocket.Conn) {
	for {
		if _, _, err := conn.Read(ctx); err != nil {
			return
		}
	}
}

type textBody struct {
	Body string `json:"body"`
}

func chatBody(data []byte) (string, bool) {
	var body textBody
	if err := json.Unmarshal(data, &body); err != nil {
		return "", false
	}
	return body.Body, true
}

func locationChannel(orderID uuid.UUID) string {
	return "order:" + orderID.String() + ":location"
}
