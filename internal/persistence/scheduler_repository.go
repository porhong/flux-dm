package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fluxdm/fluxdm/internal/download"
	"github.com/fluxdm/fluxdm/internal/scheduler"
)

type SchedulerRepository struct{ db *sql.DB }

func (d *Database) Scheduler() *SchedulerRepository { return &SchedulerRepository{db: d.db} }

func (r *SchedulerRepository) List(ctx context.Context) ([]scheduler.Schedule, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,enabled,weekdays,time_of_day,action,queue_id,speed_limit,missed_policy,post_action,created_at FROM schedules ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}
	defer rows.Close()
	result := make([]scheduler.Schedule, 0)
	for rows.Next() {
		item, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func (r *SchedulerRepository) Save(ctx context.Context, item scheduler.Schedule) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO schedules(id,name,enabled,weekdays,time_of_day,action,queue_id,speed_limit,missed_policy,post_action,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,enabled=excluded.enabled,weekdays=excluded.weekdays,time_of_day=excluded.time_of_day,action=excluded.action,queue_id=excluded.queue_id,speed_limit=excluded.speed_limit,missed_policy=excluded.missed_policy,post_action=excluded.post_action`, item.ID, item.Name, item.Enabled, encodeDays(item.Weekdays), item.TimeOfDay, item.Action, item.QueueID, item.SpeedLimit, item.MissedPolicy, item.PostAction, formatTime(item.CreatedAt))
	if err != nil {
		return fmt.Errorf("save schedule: %w", err)
	}
	return nil
}
func (r *SchedulerRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM schedules WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete schedule: %w", err)
	}
	return requireAffected(result)
}
func (r *SchedulerRepository) Claim(ctx context.Context, item scheduler.Schedule, runKey string, scheduledFor time.Time) (bool, error) {
	_, err := r.db.ExecContext(ctx, `INSERT INTO schedule_history(schedule_id,run_key,scheduled_for,executed_at,status,detail) VALUES(?,?,?,?,?,?)`, item.ID, runKey, formatTime(scheduledFor), formatTime(time.Now().UTC()), "running", "")
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return false, nil
		}
		return false, fmt.Errorf("claim schedule: %w", err)
	}
	return true, nil
}
func (r *SchedulerRepository) Finish(ctx context.Context, item scheduler.Schedule, runKey, status, detail string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE schedule_history SET status=?,detail=?,executed_at=? WHERE schedule_id=? AND run_key=? AND status='running'`, status, detail, formatTime(time.Now().UTC()), item.ID, runKey)
	if err != nil {
		return fmt.Errorf("finish schedule: %w", err)
	}
	return nil
}
func (r *SchedulerRepository) History(ctx context.Context, limit int) ([]scheduler.History, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,schedule_id,run_key,scheduled_for,executed_at,status,detail FROM schedule_history ORDER BY executed_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list schedule history: %w", err)
	}
	defer rows.Close()
	result := make([]scheduler.History, 0)
	for rows.Next() {
		var item scheduler.History
		var scheduled, executed string
		if err := rows.Scan(&item.ID, &item.ScheduleID, &item.RunKey, &scheduled, &executed, &item.Status, &item.Detail); err != nil {
			return nil, err
		}
		item.ScheduledFor, err = time.Parse(time.RFC3339Nano, scheduled)
		if err != nil {
			return nil, err
		}
		item.ExecutedAt, err = time.Parse(time.RFC3339Nano, executed)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

type scheduleScanner interface{ Scan(...any) error }

func scanSchedule(row scheduleScanner) (scheduler.Schedule, error) {
	var item scheduler.Schedule
	var days, created string
	err := row.Scan(&item.ID, &item.Name, &item.Enabled, &days, &item.TimeOfDay, &item.Action, &item.QueueID, &item.SpeedLimit, &item.MissedPolicy, &item.PostAction, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return item, download.ErrNotFound
	}
	if err != nil {
		return item, err
	}
	item.Weekdays, err = decodeDays(days)
	if err != nil {
		return item, err
	}
	item.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	return item, err
}
func encodeDays(days []int) string {
	values := append([]int(nil), days...)
	sort.Ints(values)
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = strconv.Itoa(value)
	}
	return strings.Join(parts, ",")
}
func decodeDays(value string) ([]int, error) {
	parts := strings.Split(value, ",")
	result := make([]int, 0, len(parts))
	for _, part := range parts {
		day, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("parse weekday: %w", err)
		}
		result = append(result, day)
	}
	return result, nil
}
