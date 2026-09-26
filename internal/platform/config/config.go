package config

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv        string
	HTTPAddr      string
	DatabaseURL   string
	RedisURL      string
	MigrationsDir string
	DBMaxConns    int32
	PIIKey        []byte
	PhoneHashKey  []byte
	JWTKey        []byte
	FCMServerKey  string
}

func Load() (Config, error) {
	loadDotEnv()
	cfg, err := fromEnv()
	if err != nil {
		return Config{}, err
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func loadDotEnv() {
	_ = godotenv.Load()
}

func fromEnv() (Config, error) {
	pii, hash, jwtKey, err := loadKeys()
	if err != nil {
		return Config{}, err
	}
	maxConns, err := envInt32("DB_MAX_CONNS")
	if err != nil {
		return Config{}, err
	}
	return Config{
		AppEnv:        env("APP_ENV", "development"),
		HTTPAddr:      env("HTTP_ADDR", ":8080"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		RedisURL:      env("REDIS_URL", "redis://localhost:6379/0"),
		MigrationsDir: env("MIGRATIONS_DIR", "migrations"),
		DBMaxConns:    maxConns,
		PIIKey:        pii,
		PhoneHashKey:  hash,
		JWTKey:        jwtKey,
		FCMServerKey:  os.Getenv("FCM_SERVER_KEY"),
	}, nil
}

func loadKeys() ([]byte, []byte, []byte, error) {
	pii, err := decodeKey("PII_ENCRYPTION_KEY", os.Getenv("PII_ENCRYPTION_KEY"))
	if err != nil {
		return nil, nil, nil, err
	}
	hash, err := decodeKey("PHONE_HASH_KEY", os.Getenv("PHONE_HASH_KEY"))
	if err != nil {
		return nil, nil, nil, err
	}
	jwtKey, err := decodeKey("JWT_SIGNING_KEY", os.Getenv("JWT_SIGNING_KEY"))
	if err != nil {
		return nil, nil, nil, err
	}
	return pii, hash, jwtKey, nil
}

func (c Config) validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("config: DATABASE_URL is required")
	}
	if c.HTTPAddr == "" {
		return fmt.Errorf("config: HTTP_ADDR is required")
	}
	return c.validateKeys()
}

func (c Config) validateKeys() error {
	if err := require32("PII_ENCRYPTION_KEY", c.PIIKey); err != nil {
		return err
	}
	if err := require32("PHONE_HASH_KEY", c.PhoneHashKey); err != nil {
		return err
	}
	if err := require32("JWT_SIGNING_KEY", c.JWTKey); err != nil {
		return err
	}
	return distinctKeys(c)
}

func require32(name string, key []byte) error {
	if len(key) != 32 {
		return fmt.Errorf("config: %s must be 32 bytes, base64-encoded", name)
	}
	return nil
}

func distinctKeys(c Config) error {
	if bytes.Equal(c.PIIKey, c.PhoneHashKey) {
		return fmt.Errorf("config: PII_ENCRYPTION_KEY and PHONE_HASH_KEY must differ")
	}
	if bytes.Equal(c.PIIKey, c.JWTKey) || bytes.Equal(c.PhoneHashKey, c.JWTKey) {
		return fmt.Errorf("config: JWT_SIGNING_KEY must differ from the PII keys")
	}
	return nil
}

func decodeKey(name, raw string) ([]byte, error) {
	if raw == "" {
		return nil, fmt.Errorf("config: %s is required", name)
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("config: %s must be standard base64: %w", name, err)
	}
	return key, nil
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envInt32(name string) (int32, error) {
	raw := os.Getenv(name)
	if raw == "" || raw == "0" {
		return 0, nil
	}
	return parseNonNegative(name, raw)
}

func parseNonNegative(name, raw string) (int32, error) {
	n, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("config: %s must be a non-negative integer", name)
	}
	return int32(n), nil
}
