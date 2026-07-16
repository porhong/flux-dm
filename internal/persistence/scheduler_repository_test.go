package persistence

import (
	"context"
	"github.com/fluxdm/fluxdm/internal/scheduler"
	"path/filepath"
	"testing"
	"time"
)

func TestSchedulerClaimPreventsDuplicates(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "schedule.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := db.Scheduler()
	item := scheduler.Schedule{ID: "s", Name: "Morning", Enabled: true, Weekdays: []int{1, 2}, TimeOfDay: "08:00", Action: scheduler.ActionRetryFailed, MissedPolicy: scheduler.MissedRunOnce, PostAction: scheduler.PostNone, CreatedAt: time.Now().UTC()}
	if err := repo.Save(ctx, item); err != nil {
		t.Fatal(err)
	}
	claimed, err := repo.Claim(ctx, item, "run", time.Now())
	if err != nil || !claimed {
		t.Fatalf("first claim=%v err=%v", claimed, err)
	}
	claimed, err = repo.Claim(ctx, item, "run", time.Now())
	if err != nil || claimed {
		t.Fatalf("duplicate claim=%v err=%v", claimed, err)
	}
	if err := repo.Finish(ctx, item, "run", "completed", ""); err != nil {
		t.Fatal(err)
	}
	history, err := repo.History(ctx, 10)
	if err != nil || len(history) != 1 || history[0].Status != "completed" {
		t.Fatalf("history=%#v err=%v", history, err)
	}
}
