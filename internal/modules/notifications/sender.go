package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type Notice struct {
	OrderID    uuid.UUID
	Status     string
	CustomerID uuid.UUID
	MerchantID *uuid.UUID
	RiderID    *uuid.UUID
}

type message struct {
	title string
	body  string
}

var templates = map[string]message{
	"placed":            {title: "Order placed", body: "Your order was placed."},
	"merchant_accepted": {title: "Order accepted", body: "The merchant accepted the order."},
	"preparing":         {title: "Preparing", body: "The merchant is preparing the order."},
	"ready_for_pickup":  {title: "Ready for pickup", body: "The order is ready for a rider."},
	"rider_offered":     {title: "Rider offered", body: "A rider was offered this order."},
	"accepted":          {title: "Rider assigned", body: "A rider accepted the order."},
	"picked_up":         {title: "Picked up", body: "The rider picked up the order."},
	"on_the_way":        {title: "On the way", body: "The rider is on the way."},
	"delivered":         {title: "Delivered", body: "The order was delivered."},
	"cancelled":         {title: "Cancelled", body: "The order was cancelled."},
	"rejected":          {title: "Rejected", body: "The order was rejected."},
}

type tokens interface {
	Tokens(ctx context.Context, accountID uuid.UUID) ([]string, error)
	Save(ctx context.Context, accountID uuid.UUID, role, token string) error
}

type Sender struct {
	log    zerolog.Logger
	key    string
	tokens tokens
	client *http.Client
}

func NewSender(log zerolog.Logger, key string, tokens tokens) *Sender {
	return &Sender{log: log, key: key, tokens: tokens, client: &http.Client{Timeout: 5 * time.Second}}
}

func (s *Sender) OrderStatus(ctx context.Context, notice Notice) {
	title, body := Template(notice.Status)
	s.log.Info().Str("order_id", notice.OrderID.String()).Str("status", notice.Status).Str("title", title).Msg("order notification")
	s.push(ctx, notice, title, body)
}

func Template(status string) (string, string) {
	msg, ok := templates[status]
	if !ok {
		return "Order update", "Your order was updated."
	}
	return msg.title, msg.body
}

func (s *Sender) push(ctx context.Context, notice Notice, title, body string) {
	if s.key == "" || s.tokens == nil {
		return
	}
	for _, id := range accounts(notice) {
		s.sendAccount(ctx, id, title, body)
	}
}

func (s *Sender) sendAccount(ctx context.Context, accountID uuid.UUID, title, body string) {
	found, err := s.tokens.Tokens(ctx, accountID)
	if err != nil {
		s.log.Error().Err(err).Str("account_id", accountID.String()).Msg("notification tokens")
		return
	}
	for _, token := range found {
		s.post(ctx, token, title, body)
	}
}

func (s *Sender) post(ctx context.Context, token, title, body string) {
	req, err := fcmRequest(ctx, s.key, token, title, body)
	if err != nil {
		return
	}
	resp, err := s.client.Do(req)
	if err != nil {
		s.log.Error().Err(err).Msg("fcm send")
		return
	}
	s.finishPost(resp)
}

func fcmRequest(ctx context.Context, key, token, title, body string) (*http.Request, error) {
	payload, err := json.Marshal(map[string]any{
		"to":           token,
		"notification": map[string]string{"title": title, "body": body},
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://fcm.googleapis.com/fcm/send", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "key="+key)
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func (s *Sender) finishPost(resp *http.Response) {
	_ = resp.Body.Close()
	if resp.StatusCode >= 300 {
		s.log.Error().Int("status", resp.StatusCode).Msg("fcm send")
	}
}

func (s *Sender) Save(ctx context.Context, accountID uuid.UUID, role, token string) error {
	if s.tokens == nil {
		return nil
	}
	return s.tokens.Save(ctx, accountID, role, token)
}

func accounts(notice Notice) []uuid.UUID {
	out := []uuid.UUID{notice.CustomerID}
	if notice.MerchantID != nil {
		out = append(out, *notice.MerchantID)
	}
	if notice.RiderID != nil {
		out = append(out, *notice.RiderID)
	}
	return out
}
