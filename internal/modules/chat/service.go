package chat

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/yourusername/ghartak-backend/internal/modules/orders"
	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
)

type parties interface {
	Party(ctx context.Context, orderID uuid.UUID) (orders.Party, error)
}

type Service struct {
	store   *Repository
	parties parties
	redis   *redis.Client
	log     zerolog.Logger
}

func NewService(store *Repository, parties parties, redis *redis.Client, log zerolog.Logger) *Service {
	return &Service{store: store, parties: parties, redis: redis, log: log}
}

func (s *Service) Send(ctx context.Context, role string, senderID, orderID uuid.UUID, body string) (Message, error) {
	if err := validBody(body); err != nil {
		return Message{}, err
	}
	if err := s.authorize(ctx, role, senderID, orderID); err != nil {
		return Message{}, err
	}
	msg, err := s.store.Insert(ctx, orderID, role, senderID, body)
	if err != nil {
		return Message{}, err
	}
	s.publish(ctx, msg)
	return msg, nil
}

func (s *Service) List(ctx context.Context, role string, senderID, orderID uuid.UUID) ([]Message, error) {
	if err := s.authorize(ctx, role, senderID, orderID); err != nil {
		return nil, err
	}
	return s.store.List(ctx, orderID)
}

func (s *Service) authorize(ctx context.Context, role string, senderID, orderID uuid.UUID) error {
	party, err := s.parties.Party(ctx, orderID)
	if err != nil {
		return err
	}
	if !party.Allows(role, senderID) {
		return apperror.Forbidden("not a party to this order")
	}
	return nil
}

func (s *Service) publish(ctx context.Context, msg Message) {
	if s.redis == nil {
		return
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}
	if err := s.redis.Publish(ctx, Channel(msg.OrderID), payload).Err(); err != nil {
		s.log.Error().Err(err).Str("order_id", msg.OrderID.String()).Msg("chat publish")
	}
}

func Channel(orderID uuid.UUID) string {
	return "order:" + orderID.String() + ":chat"
}

func validBody(body string) error {
	if len(body) < 1 || len(body) > 1000 {
		return apperror.Invalid("message is invalid")
	}
	return nil
}
