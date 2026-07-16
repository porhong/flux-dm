package persistence

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenAppliesMigrationsOnce(t *testing.T) {
	ctx := context.Background()
	database, err := Open(ctx, t.TempDir()+"/fluxdm-test.db")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer database.Close()

	var migrations int
	if err := database.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&migrations); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if migrations != 8 {
		t.Fatalf("expected eight migrations, got %d", migrations)
	}
	if err := database.migrate(ctx); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
}

func TestOpenRecoveringPreservesCorruptDatabase(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "fluxdm.db")
	if err := os.WriteFile(path, []byte("not a sqlite database"), 0o600); err != nil {
		t.Fatal(err)
	}
	database, recovery, err := OpenRecovering(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if recovery.BackupPath == "" {
		t.Fatal("expected backup path")
	}
	payload, err := os.ReadFile(recovery.BackupPath)
	if err != nil || string(payload) != "not a sqlite database" {
		t.Fatalf("backup=%q err=%v", payload, err)
	}
	if err := database.Ping(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestClearPrivateDataKeepsDownloadedFilesOutOfScope(t *testing.T) {
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "private.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := database.db.ExecContext(ctx, `INSERT INTO downloads(id,url,file_name,destination_path,temp_path,state,created_at)VALUES('done','https://example.test/a','a','C:\\Downloads\\a','C:\\Downloads\\a.part','completed','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := database.ClearPrivateData(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := database.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM downloads`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}
