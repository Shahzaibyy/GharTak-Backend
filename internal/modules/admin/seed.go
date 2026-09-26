package admin

import (
	"context"

	"github.com/yourusername/ghartak-backend/internal/platform/pii"
)

const demoAdminPhone = "+923000000002"

func (r *Repository) SeedDemoAdmin(ctx context.Context, encKey, hashKey []byte) error {
	ciphertext, lookup, err := pii.SealPhone(encKey, hashKey, demoAdminPhone)
	if err != nil {
		return err
	}
	return r.EnsureAdmin(ctx, ciphertext, lookup, "GharTak Platform")
}
