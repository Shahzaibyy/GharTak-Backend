package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const advisoryLockKey int64 = 74251001

var versionPattern = regexp.MustCompile(`^[0-9]{4}_[a-z0-9_]+$`)

func Migrate(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("migrate: acquire: %w", err)
	}
	defer conn.Release()
	if err := lock(ctx, conn); err != nil {
		return err
	}
	defer unlock(ctx, conn)
	return migrateLocked(ctx, pool, dir)
}

func lock(ctx context.Context, conn *pgxpool.Conn) error {
	_, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, advisoryLockKey)
	if err != nil {
		return fmt.Errorf("migrate: lock: %w", err)
	}
	return nil
}

func unlock(ctx context.Context, conn *pgxpool.Conn) {
	_, _ = conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, advisoryLockKey)
}

func migrateLocked(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	if err := ensureSchemaTable(ctx, pool); err != nil {
		return err
	}
	names, err := upFiles(dir)
	if err != nil {
		return err
	}
	return applyAll(ctx, pool, dir, names)
}

func ensureSchemaTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`)
	if err != nil {
		return fmt.Errorf("migrate: schema_migrations: %w", err)
	}
	return nil
}

func upFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("migrate: read dir: %w", err)
	}
	return filterUp(entries), nil
}

func filterUp(entries []os.DirEntry) []string {
	names := make([]string, 0)
	for _, entry := range entries {
		if keepUp(entry) {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names
}

func keepUp(entry os.DirEntry) bool {
	if entry.IsDir() {
		return false
	}
	return strings.HasSuffix(entry.Name(), ".up.sql")
}

func applyAll(ctx context.Context, pool *pgxpool.Pool, dir string, names []string) error {
	for _, name := range names {
		if err := applyFile(ctx, pool, dir, name); err != nil {
			return err
		}
	}
	return nil
}

func applyFile(ctx context.Context, pool *pgxpool.Pool, dir, name string) error {
	version := strings.TrimSuffix(name, ".up.sql")
	if !versionPattern.MatchString(version) {
		return fmt.Errorf("migrate: invalid filename %s", name)
	}
	done, err := alreadyApplied(ctx, pool, version)
	if err != nil {
		return err
	}
	if done {
		return nil
	}
	return applyFresh(ctx, pool, dir, name, version)
}

func alreadyApplied(ctx context.Context, pool *pgxpool.Pool, version string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("migrate: check version: %w", err)
	}
	return exists, nil
}

func applyFresh(ctx context.Context, pool *pgxpool.Pool, dir, name, version string) error {
	body, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return fmt.Errorf("migrate: read %s: %w", name, err)
	}
	stmts, err := splitSQL(string(body))
	if err != nil {
		return err
	}
	return runMigration(ctx, pool, version, stmts)
}

func runMigration(ctx context.Context, pool *pgxpool.Pool, version string, stmts []string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("migrate: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := execAll(ctx, tx, version, stmts); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("migrate: commit: %w", err)
	}
	return nil
}

func execAll(ctx context.Context, tx pgx.Tx, version string, stmts []string) error {
	if err := execStmts(ctx, tx, stmts); err != nil {
		return err
	}
	return insertVersion(ctx, tx, version)
}

func execStmts(ctx context.Context, tx pgx.Tx, stmts []string) error {
	for _, stmt := range stmts {
		if _, err := tx.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("migrate: exec: %w", err)
		}
	}
	return nil
}

func insertVersion(ctx context.Context, tx pgx.Tx, version string) error {
	_, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version)
	if err != nil {
		return fmt.Errorf("migrate: record version: %w", err)
	}
	return nil
}
