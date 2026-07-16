package scheduler

import (
	"testing"
	"time"
)

func TestDueMissedPolicyAndRunKey(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 30, 0, 0, time.Local)
	base := Schedule{ID: "a", Name: "test", Enabled: true, Weekdays: []int{int(now.Weekday())}, TimeOfDay: "12:00", Action: ActionRetryFailed, MissedPolicy: MissedRunOnce, PostAction: PostNone}
	got := Due(now, []Schedule{base})
	if len(got) != 1 || !got[0].Missed || got[0].RunKey == "" {
		t.Fatalf("unexpected due occurrence: %#v", got)
	}
	base.MissedPolicy = MissedSkip
	if got := Due(now, []Schedule{base}); len(got) != 0 {
		t.Fatalf("skip policy ran missed occurrence")
	}
}

func TestValidateRequiresExplicitPowerActionValue(t *testing.T) {
	item := Schedule{Name: "x", Weekdays: []int{1}, TimeOfDay: "08:00", Action: ActionRetryFailed, MissedPolicy: MissedSkip, PostAction: "surprise"}
	if Validate(item) == nil {
		t.Fatal("expected invalid post action")
	}
}
