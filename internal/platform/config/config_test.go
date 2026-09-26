package config_test

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/yourusername/ghartak-backend/internal/platform/config"
)

func TestLoadRequiresDatabase(t *testing.T) {
	setKeys(t)
	t.Setenv("DATABASE_URL", "")
	if _, err := config.Load(); err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadRejectsMatchingKeys(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(bytes32("same-key-for-both-purposes!!!!"))
	t.Setenv("DATABASE_URL", "postgres://localhost/ghartak")
	t.Setenv("PII_ENCRYPTION_KEY", key)
	t.Setenv("PHONE_HASH_KEY", key)
	t.Setenv("JWT_SIGNING_KEY", base64.StdEncoding.EncodeToString(bytes32("jwt-key-for-config-test!!!!!!")))
	if _, err := config.Load(); err == nil || !strings.Contains(err.Error(), "must differ") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoad(t *testing.T) {
	setKeys(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/ghartak")
	t.Setenv("HTTP_ADDR", ":9090")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":9090" || len(cfg.PIIKey) != 32 || len(cfg.PhoneHashKey) != 32 {
		t.Fatalf("cfg addr %s key lens %d %d", cfg.HTTPAddr, len(cfg.PIIKey), len(cfg.PhoneHashKey))
	}
}

func setKeys(t *testing.T) {
	t.Helper()
	t.Setenv("PII_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(bytes32("pii-key-for-config-test!!!!!!")))
	t.Setenv("PHONE_HASH_KEY", base64.StdEncoding.EncodeToString(bytes32("hash-key-for-config-test!!!!!")))
	t.Setenv("JWT_SIGNING_KEY", base64.StdEncoding.EncodeToString(bytes32("jwt-key-for-config-test!!!!!!")))
}

func bytes32(raw string) []byte {
	buf := make([]byte, 32)
	copy(buf, raw)
	return buf
}
