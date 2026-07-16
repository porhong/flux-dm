package application

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fluxdm/fluxdm/internal/persistence"
	"github.com/fluxdm/fluxdm/internal/scheduler"
)

type recordingScheduleActions struct {
	mu                      sync.Mutex
	executions, postActions int
}

func (a *recordingScheduleActions) ExecuteSchedule(context.Context, scheduler.Schedule) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.executions++
	return nil
}
func (a *recordingScheduleActions) ExecutePostAction(context.Context, scheduler.Schedule) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.postActions++
	return nil
}

func TestSchedulerServiceClaimsBeforeExecution(t *testing.T) {
	ctx := context.Background()
	database, err := persistence.Open(ctx, filepath.Join(t.TempDir(), "scheduler.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	actions := &recordingScheduleActions{}
	service := NewSchedulerService(ctx, database.Scheduler(), actions)
	defer service.Close()
	now := time.Now()
	_, err = service.Save(ctx, SaveScheduleInput{Name: "Once", Enabled: true, Weekdays: []int{int(now.Weekday())}, TimeOfDay: now.Format("15:04"), Action: scheduler.ActionRetryFailed, MissedPolicy: scheduler.MissedRunOnce, PostAction: scheduler.PostExit})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunDue(ctx, now); err != nil {
		t.Fatal(err)
	}
	if err := service.RunDue(ctx, now); err != nil {
		t.Fatal(err)
	}
	actions.mu.Lock()
	defer actions.mu.Unlock()
	if actions.executions != 1 || actions.postActions != 1 {
		t.Fatalf("executions=%d post=%d", actions.executions, actions.postActions)
	}
}

func TestSchedulerServiceRequiresPowerActionConfirmation(t *testing.T) {
	ctx := context.Background()
	database, err := persistence.Open(ctx, filepath.Join(t.TempDir(), "scheduler-confirm.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := NewSchedulerService(ctx, database.Scheduler(), &recordingScheduleActions{})
	defer service.Close()
	input := SaveScheduleInput{
		Name:         "Power after completion",
		Enabled:      true,
		Weekdays:     []int{1},
		TimeOfDay:    "08:00",
		Action:       scheduler.ActionRetryFailed,
		MissedPolicy: scheduler.MissedSkip,
		PostAction:   scheduler.PostShutdown,
	}
	if _, err := service.Save(ctx, input); err == nil {
		t.Fatal("saved a shutdown schedule without explicit confirmation")
	}
	input.ConfirmPowerAction = true
	if _, err := service.Save(ctx, input); err != nil {
		t.Fatalf("save confirmed shutdown schedule: %v", err)
	}
}
