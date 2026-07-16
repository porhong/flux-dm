package application

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/fluxdm/fluxdm/internal/organization"
	"github.com/fluxdm/fluxdm/internal/scheduler"
)

type SchedulerActions interface {
	ExecuteSchedule(context.Context, scheduler.Schedule) error
	ExecutePostAction(context.Context, scheduler.Schedule) error
}

type SaveScheduleInput struct {
	ID                 string                 `json:"id"`
	Name               string                 `json:"name"`
	Enabled            bool                   `json:"enabled"`
	Weekdays           []int                  `json:"weekdays"`
	TimeOfDay          string                 `json:"timeOfDay"`
	Action             scheduler.Action       `json:"action"`
	QueueID            string                 `json:"queueId"`
	SpeedLimit         int64                  `json:"speedLimit"`
	MissedPolicy       scheduler.MissedPolicy `json:"missedPolicy"`
	PostAction         scheduler.PostAction   `json:"postAction"`
	ConfirmPowerAction bool                   `json:"confirmPowerAction"`
}

type SchedulerService struct {
	repository   scheduler.Repository
	organization organization.Repository
	actions      SchedulerActions
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

func NewSchedulerService(parent context.Context, repository scheduler.Repository, actions SchedulerActions, organizations ...organization.Repository) *SchedulerService {
	ctx, cancel := context.WithCancel(parent)
	service := &SchedulerService{repository: repository, actions: actions, ctx: ctx, cancel: cancel}
	if len(organizations) > 0 {
		service.organization = organizations[0]
	}
	service.wg.Add(1)
	go service.loop()
	return service
}
func (s *SchedulerService) Close() { s.cancel(); s.wg.Wait() }
func (s *SchedulerService) List(ctx context.Context) ([]scheduler.Schedule, error) {
	items, err := s.repository.List(ctx)
	if err != nil {
		return nil, NewError(ErrInternal, "Could not list schedules.", err)
	}
	return items, nil
}
func (s *SchedulerService) History(ctx context.Context, limit int) ([]scheduler.History, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		return nil, NewError(ErrInvalidInput, "History limit cannot exceed 1000.", nil)
	}
	items, err := s.repository.History(ctx, limit)
	if err != nil {
		return nil, NewError(ErrInternal, "Could not list schedule history.", err)
	}
	return items, nil
}
func (s *SchedulerService) Save(ctx context.Context, input SaveScheduleInput) (scheduler.Schedule, error) {
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = newID()
	} else if _, err := validateID(id); err != nil {
		return scheduler.Schedule{}, NewError(ErrInvalidInput, "Invalid schedule identifier.", err)
	}
	item := scheduler.Schedule{ID: id, Name: strings.TrimSpace(input.Name), Enabled: input.Enabled, Weekdays: append([]int(nil), input.Weekdays...), TimeOfDay: strings.TrimSpace(input.TimeOfDay), Action: input.Action, QueueID: strings.TrimSpace(input.QueueID), SpeedLimit: input.SpeedLimit, MissedPolicy: input.MissedPolicy, PostAction: input.PostAction, CreatedAt: time.Now().UTC()}
	if isPowerPostAction(item.PostAction) && !input.ConfirmPowerAction {
		return scheduler.Schedule{}, NewError(ErrInvalidInput, "Confirm the selected system power action.", nil)
	}
	if err := scheduler.Validate(item); err != nil {
		return scheduler.Schedule{}, NewError(ErrInvalidInput, "Check the schedule settings.", err)
	}
	if s.organization != nil && (item.Action == scheduler.ActionStartQueue || item.Action == scheduler.ActionStopQueue) {
		if _, err := s.organization.GetQueue(ctx, item.QueueID); err != nil {
			return scheduler.Schedule{}, NewError(ErrInvalidInput, "Choose an existing queue.", err)
		}
	}
	if err := s.repository.Save(ctx, item); err != nil {
		return scheduler.Schedule{}, NewError(ErrInternal, "Could not save schedule.", err)
	}
	return item, nil
}

func isPowerPostAction(action scheduler.PostAction) bool {
	return action == scheduler.PostSleep || action == scheduler.PostHibernate || action == scheduler.PostShutdown
}
func (s *SchedulerService) Delete(ctx context.Context, id string) error {
	id, err := validateID(id)
	if err != nil {
		return NewError(ErrInvalidInput, "Invalid schedule identifier.", err)
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return repositoryError("delete schedule", err)
	}
	return nil
}
func (s *SchedulerService) RunDue(ctx context.Context, now time.Time) error {
	items, err := s.repository.List(ctx)
	if err != nil {
		return err
	}
	for _, occurrence := range scheduler.Due(now, items) {
		claimed, claimErr := s.repository.Claim(ctx, occurrence.Schedule, occurrence.RunKey, occurrence.ScheduledFor)
		if claimErr != nil {
			return claimErr
		}
		if !claimed {
			continue
		}
		status, detail := "completed", ""
		if executeErr := s.actions.ExecuteSchedule(ctx, occurrence.Schedule); executeErr != nil {
			status = "failed"
			detail = executeErr.Error()
		}
		if finishErr := s.repository.Finish(ctx, occurrence.Schedule, occurrence.RunKey, status, detail); finishErr != nil {
			return finishErr
		}
		if status == "completed" && occurrence.Schedule.PostAction != scheduler.PostNone {
			if err := s.actions.ExecutePostAction(ctx, occurrence.Schedule); err != nil {
				return fmt.Errorf("post action: %w", err)
			}
		}
	}
	return nil
}
func (s *SchedulerService) loop() {
	defer s.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	_ = s.RunDue(s.ctx, time.Now())
	for {
		select {
		case <-s.ctx.Done():
			return
		case now := <-ticker.C:
			_ = s.RunDue(s.ctx, now)
		}
	}
}
