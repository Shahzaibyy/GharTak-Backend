package support

import (
	"context"

	"github.com/google/uuid"
)

type Service struct {
	store *Repository
}

func NewService(store *Repository) *Service {
	return &Service{store: store}
}

func (s *Service) Rate(ctx context.Context, in RatingInput) error {
	if err := validScore(in); err != nil {
		return err
	}
	return s.store.Rate(ctx, in)
}

func (s *Service) Open(ctx context.Context, customerID, orderID uuid.UUID, body string) (Ticket, error) {
	if err := validBody(body, 2000); err != nil {
		return Ticket{}, err
	}
	return s.store.Open(ctx, customerID, orderID, body)
}

func (s *Service) List(ctx context.Context, customerID uuid.UUID) ([]Ticket, error) {
	return s.store.List(ctx, customerID)
}

func (s *Service) Get(ctx context.Context, customerID, ticketID uuid.UUID) (Ticket, error) {
	return s.store.Get(ctx, customerID, ticketID)
}

func (s *Service) Messages(ctx context.Context, customerID, ticketID uuid.UUID) ([]Message, error) {
	return s.store.Messages(ctx, customerID, ticketID)
}

func (s *Service) Reply(ctx context.Context, customerID, ticketID uuid.UUID, role string, senderID uuid.UUID, body string) (Message, error) {
	if err := validBody(body, 2000); err != nil {
		return Message{}, err
	}
	return s.store.Reply(ctx, customerID, ticketID, role, senderID, body)
}
