package orders

import (
	"context"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/domain"
)

var merchantActs = map[string]Status{
	"accept":    StatusMerchantAccepted,
	"preparing": StatusPreparing,
	"ready":     StatusReadyForPickup,
}

var riderActs = map[string]Status{
	"pickup":  StatusPickedUp,
	"enroute": StatusOnTheWay,
}

func (s *Service) MerchantAction(ctx context.Context, merchantID, orderID uuid.UUID, action string) (Order, error) {
	to, ok := merchantActs[action]
	if !ok {
		return Order{}, apperror.Invalid("action is invalid")
	}
	order, err := s.orders.Advance(ctx, advance{
		OrderID: orderID, To: to, ActorRole: string(domain.RoleMerchant), ActorID: merchantID, ExpectMerchant: &merchantID,
	})
	if err != nil {
		return Order{}, err
	}
	return s.afterStatus(ctx, order, to == StatusReadyForPickup)
}

func (s *Service) RiderAction(ctx context.Context, riderID, orderID uuid.UUID, action string) (Order, error) {
	to, ok := riderActs[action]
	if !ok {
		return Order{}, apperror.Invalid("action is invalid")
	}
	order, err := s.orders.Advance(ctx, advance{
		OrderID: orderID, To: to, ActorRole: string(domain.RoleRider), ActorID: riderID, ExpectRider: &riderID,
	})
	if err != nil {
		return Order{}, err
	}
	s.notify(ctx, order)
	return order, nil
}

func (s *Service) Accept(ctx context.Context, riderID, orderID uuid.UUID) (Order, error) {
	if _, err := s.orders.AcceptOffer(ctx, riderID, orderID); err != nil {
		return Order{}, err
	}
	return s.changed(ctx, orderID)
}

func (s *Service) Reject(ctx context.Context, riderID, orderID uuid.UUID) (Order, error) {
	order, err := s.orders.RejectOffer(ctx, riderID, orderID)
	if err != nil {
		return Order{}, err
	}
	_ = s.offer(ctx, orderID)
	s.notify(ctx, order)
	return order, nil
}

func (s *Service) Deliver(ctx context.Context, riderID, orderID uuid.UUID, otp, proof string) (Order, error) {
	order, err := s.orders.Deliver(ctx, deliverInput{
		RiderID: riderID, OrderID: orderID, OTP: otp, Proof: proof, HashKey: s.otpKey,
	})
	if err != nil {
		return Order{}, err
	}
	return s.changed(ctx, order.ID)
}

func (s *Service) Cancel(ctx context.Context, customerID, orderID uuid.UUID, reason string) (Order, error) {
	if _, err := s.orders.Cancel(ctx, customerID, orderID, reason); err != nil {
		return Order{}, err
	}
	return s.changed(ctx, orderID)
}

func (s *Service) DispatchFor(ctx context.Context, role string, actor, orderID uuid.UUID) (Order, error) {
	party, err := s.orders.Party(ctx, orderID)
	if err != nil {
		return Order{}, err
	}
	if !party.Allows(role, actor) {
		return Order{}, apperror.Forbidden("not a party to this order")
	}
	if err := s.offer(ctx, orderID); err != nil {
		return Order{}, err
	}
	return s.orders.GetByID(ctx, orderID)
}

func (s *Service) ListMerchant(ctx context.Context, merchantID uuid.UUID, status Status) ([]Order, error) {
	return s.orders.ListByMerchant(ctx, merchantID, status)
}

func (s *Service) ListOffers(ctx context.Context, riderID uuid.UUID) ([]Order, error) {
	return s.orders.ListOffers(ctx, riderID)
}

func (s *Service) ListTasks(ctx context.Context, riderID uuid.UUID) ([]Order, error) {
	return s.orders.ListTasks(ctx, riderID)
}

func (s *Service) Party(ctx context.Context, orderID uuid.UUID) (Party, error) {
	return s.orders.Party(ctx, orderID)
}

func (s *Service) afterStatus(ctx context.Context, order Order, offer bool) (Order, error) {
	s.notify(ctx, order)
	if !offer {
		return order, nil
	}
	return s.reload(ctx, order)
}

func (s *Service) reload(ctx context.Context, order Order) (Order, error) {
	// Auto-dispatch on ready: soft-fail so merchant advance still succeeds
	// (manual POST /orders/{id}/dispatch returns a clear error if no riders).
	if err := s.offer(ctx, order.ID); err != nil {
		return order, nil
	}
	fresh, err := s.orders.GetByID(ctx, order.ID)
	if err != nil {
		return order, nil
	}
	fresh.DeliveryOTP = order.DeliveryOTP
	return fresh, nil
}

func (s *Service) changed(ctx context.Context, orderID uuid.UUID) (Order, error) {
	order, err := s.orders.GetByID(ctx, orderID)
	if err != nil {
		return Order{}, err
	}
	s.notify(ctx, order)
	return order, nil
}
