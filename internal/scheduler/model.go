package scheduler

import (
	"context"
	"fmt"
	"sort"
	"time"
)

type Action string

const (
	ActionStartQueue   Action = "start_queue"
	ActionStopQueue    Action = "stop_queue"
	ActionSpeedProfile Action = "speed_profile"
	ActionRetryFailed  Action = "retry_failed"
)

type MissedPolicy string

const (
	MissedSkip    MissedPolicy = "skip"
	MissedRunOnce MissedPolicy = "run_once"
)

type PostAction string

const (
	PostNone      PostAction = "none"
	PostExit      PostAction = "exit"
	PostSleep     PostAction = "sleep"
	PostHibernate PostAction = "hibernate"
	PostShutdown  PostAction = "shutdown"
)

type Schedule struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Enabled      bool         `json:"enabled"`
	Weekdays     []int        `json:"weekdays"`
	TimeOfDay    string       `json:"timeOfDay"`
	Action       Action       `json:"action"`
	QueueID      string       `json:"queueId"`
	SpeedLimit   int64        `json:"speedLimit"`
	MissedPolicy MissedPolicy `json:"missedPolicy"`
	PostAction   PostAction   `json:"postAction"`
	CreatedAt    time.Time    `json:"createdAt"`
}

type History struct {
	ID           int64     `json:"id"`
	ScheduleID   string    `json:"scheduleId"`
	RunKey       string    `json:"runKey"`
	ScheduledFor time.Time `json:"scheduledFor"`
	ExecutedAt   time.Time `json:"executedAt"`
	Status       string    `json:"status"`
	Detail       string    `json:"detail"`
}

type Repository interface {
	List(context.Context) ([]Schedule, error)
	Save(context.Context, Schedule) error
	Delete(context.Context, string) error
	Claim(context.Context, Schedule, string, time.Time) (bool, error)
	Finish(context.Context, Schedule, string, string, string) error
	History(context.Context, int) ([]History, error)
}

type DueOccurrence struct {
	Schedule     Schedule
	RunKey       string
	ScheduledFor time.Time
	Missed       bool
}

func Due(now time.Time, schedules []Schedule) []DueOccurrence {
	result := make([]DueOccurrence, 0)
	for _, item := range schedules {
		if !item.Enabled || !containsDay(item.Weekdays, int(now.Weekday())) {
			continue
		}
		hour, minute, err := parseClock(item.TimeOfDay)
		if err != nil {
			continue
		}
		occurrence := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		if now.Before(occurrence) {
			continue
		}
		missed := now.Sub(occurrence) >= time.Minute
		if missed && item.MissedPolicy != MissedRunOnce {
			continue
		}
		result = append(result, DueOccurrence{Schedule: item, RunKey: occurrence.Format("2006-01-02T15:04Z07:00"), ScheduledFor: occurrence, Missed: missed})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ScheduledFor.Before(result[j].ScheduledFor) })
	return result
}

func Validate(item Schedule) error {
	if item.Name == "" || len(item.Name) > 100 {
		return fmt.Errorf("invalid schedule name")
	}
	if _, _, err := parseClock(item.TimeOfDay); err != nil {
		return err
	}
	if len(item.Weekdays) == 0 || len(item.Weekdays) > 7 {
		return fmt.Errorf("choose weekdays")
	}
	for _, day := range item.Weekdays {
		if day < 0 || day > 6 {
			return fmt.Errorf("invalid weekday")
		}
	}
	switch item.Action {
	case ActionStartQueue, ActionStopQueue, ActionSpeedProfile, ActionRetryFailed:
	default:
		return fmt.Errorf("invalid action")
	}
	if (item.Action == ActionStartQueue || item.Action == ActionStopQueue) && item.QueueID == "" {
		return fmt.Errorf("queue is required")
	}
	if item.SpeedLimit < 0 {
		return fmt.Errorf("invalid speed limit")
	}
	if item.MissedPolicy != MissedSkip && item.MissedPolicy != MissedRunOnce {
		return fmt.Errorf("invalid missed policy")
	}
	switch item.PostAction {
	case PostNone, PostExit, PostSleep, PostHibernate, PostShutdown:
	default:
		return fmt.Errorf("invalid post action")
	}
	return nil
}

func parseClock(value string) (int, int, error) {
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return 0, 0, fmt.Errorf("time must use HH:MM: %w", err)
	}
	return parsed.Hour(), parsed.Minute(), nil
}
func containsDay(days []int, value int) bool {
	for _, day := range days {
		if day == value {
			return true
		}
	}
	return false
}
