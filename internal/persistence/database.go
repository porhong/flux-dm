package persistence

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Database struct {
	db *sql.DB
}

func Open(ctx context.Context, path string) (*Database, error) {
	if path != ":memory:" {
		path = filepath.Clean(path)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	database := &Database{db: db}
	if err := database.configure(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := database.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return database, nil
}

type Recovery struct{ BackupPath string }

func OpenRecovering(ctx context.Context, path string) (*Database, Recovery, error) {
	database, err := Open(ctx, path)
	if err == nil {
		return database, Recovery{}, nil
	}
	if path == ":memory:" || !isCorruptionError(err) {
		return nil, Recovery{}, err
	}
	absolute, absErr := filepath.Abs(path)
	if absErr != nil {
		return nil, Recovery{}, err
	}
	backup := absolute + ".corrupt-" + time.Now().UTC().Format("20060102T150405.000000000Z") + ".bak"
	if renameErr := os.Rename(absolute, backup); renameErr != nil {
		return nil, Recovery{}, fmt.Errorf("preserve corrupt database: %w", renameErr)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, statErr := os.Stat(absolute + suffix); statErr == nil {
			_ = os.Rename(absolute+suffix, backup+suffix)
		}
	}
	database, openErr := Open(ctx, absolute)
	if openErr != nil {
		return nil, Recovery{BackupPath: backup}, openErr
	}
	return database, Recovery{BackupPath: backup}, nil
}
func isCorruptionError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "not a database") || strings.Contains(message, "database disk image is malformed") || strings.Contains(message, "database corrupt")
}

func (d *Database) configure(ctx context.Context) error {
	for _, statement := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
	} {
		if _, err := d.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("configure sqlite: %w", err)
		}
	}
	return nil
}

func (d *Database) migrate(ctx context.Context) error {
	if _, err := d.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}

	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		if err := d.applyMigration(ctx, entry.Name()); err != nil {
			return err
		}
	}
	return nil
}

func (d *Database) applyMigration(ctx context.Context, name string) error {
	var applied bool
	if err := d.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = ?)", name).Scan(&applied); err != nil {
		return fmt.Errorf("check migration %s: %w", name, err)
	}
	if applied {
		return nil
	}

	script, err := migrationFiles.ReadFile("migrations/" + name)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", name, err)
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", name, err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, string(script)); err != nil {
		return fmt.Errorf("apply migration %s: %w", name, err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations(version) VALUES (?)", name); err != nil {
		return fmt.Errorf("record migration %s: %w", name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", name, err)
	}
	return nil
}

func (d *Database) Ping(ctx context.Context) error { return d.db.PingContext(ctx) }

func (d *Database) ClearPrivateData(ctx context.Context) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range []string{`DELETE FROM download_secrets`, `DELETE FROM site_profiles`, `DELETE FROM schedule_history`, `DELETE FROM downloads WHERE state IN ('completed','failed','cancelled')`} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("clear private data: %w", err)
		}
	}
	return tx.Commit()
}

func (d *Database) Close() error { return d.db.Close() }
