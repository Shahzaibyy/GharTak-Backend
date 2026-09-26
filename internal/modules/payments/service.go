package payments

import "context"

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreditWallet(ctx context.Context, in AdjustInput) error {
	return s.repo.CreditWallet(ctx, in)
}

func (s *Service) SettleCash(ctx context.Context, in SettleInput) error {
	return s.repo.SettleCash(ctx, in)
}

func (s *Service) ApplyWebhook(ctx context.Context, ref string, status Status) error {
	return s.repo.ApplyWebhook(ctx, ref, status)
}
