package persistence

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/fluxdm/fluxdm/internal/organization"
)

func TestOrganizationRepositoryRoundTrip(t *testing.T) {
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "organization.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := database.Organization()
	now := time.Now().UTC().Truncate(time.Microsecond)
	category := organization.Category{ID: "cat", Name: "Archives", Extensions: []string{"ZIP", "7z"}, DestinationDir: t.TempDir(), Priority: 4, CreatedAt: now}
	if err := repository.SaveCategory(ctx, category); err != nil {
		t.Fatal(err)
	}
	categories, err := repository.ListCategories(ctx)
	if err != nil || len(categories) != 1 || categories[0].Extensions[1] != "zip" {
		t.Fatalf("categories=%#v err=%v", categories, err)
	}
	queue := organization.Queue{ID: "queue", Name: "Night", Priority: 3, MaxParallel: 2, MaxConnections: 8, BandwidthLimit: 1024, Sequential: false, Enabled: true, CreatedAt: now}
	if err := repository.SaveQueue(ctx, queue); err != nil {
		t.Fatal(err)
	}
	actual, err := repository.GetQueue(ctx, "queue")
	if err != nil || actual.MaxParallel != 2 || actual.MaxConnections != 8 || !actual.Enabled {
		t.Fatalf("queue=%#v err=%v", actual, err)
	}
}
