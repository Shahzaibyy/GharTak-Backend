package admin_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yourusername/ghartak-backend/internal/modules/admin"
	"github.com/yourusername/ghartak-backend/internal/platform/database"
)

func TestRepositoryListZones(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := database.NewPool(ctx, database.PoolConfig{URL: url})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool, filepath.Join("..", "..", "..", "migrations")); err != nil {
		t.Fatal(err)
	}
	repo := admin.NewRepository(pool)
	all, err := repo.List(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	active, err := repo.List(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != len(admin.ZoneSeeds()) {
		t.Fatalf("zones = %d", len(all))
	}
	if len(active) != 2 {
		t.Fatalf("active = %d", len(active))
	}
	if all[0].BaseDeliveryFee.String() != admin.SeedBaseDeliveryFee {
		t.Fatalf("fee = %s", all[0].BaseDeliveryFee)
	}
}
