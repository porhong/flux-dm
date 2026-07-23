package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/fluxdm/fluxdm/internal/download"
)

type DownloadRepository struct{ db *sql.DB }

func (d *Database) Downloads() *DownloadRepository { return &DownloadRepository{db: d.db} }

func (r *DownloadRepository) Create(ctx context.Context, task download.Download) error {
	task.Segments = normalizeSegments(task)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin create download: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO downloads (
		id, url, final_url, file_name, destination_path, temp_path, state,
		total_bytes, downloaded_bytes, range_supported, etag, last_modified,
		mime_type, created_at, started_at, completed_at, last_error, retry_count, connections, bandwidth_limit,
		category_id, queue_id, queue_position, priority, site_profile_id
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, downloadValues(task)...); err != nil {
		return fmt.Errorf("create download: %w", err)
	}
	if err := insertSegments(ctx, tx, task.Segments); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit create download: %w", err)
	}
	return nil
}

func (r *DownloadRepository) Get(ctx context.Context, id string) (download.Download, error) {
	task, err := scanDownload(r.db.QueryRowContext(ctx, selectDownload+` WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return download.Download{}, download.ErrNotFound
	}
	if err != nil {
		return download.Download{}, fmt.Errorf("get download: %w", err)
	}
	task.Segments, err = loadSegments(ctx, r.db, task.ID)
	if err != nil {
		return download.Download{}, err
	}
	return task, nil
}

func (r *DownloadRepository) List(ctx context.Context) ([]download.Download, error) {
	rows, err := r.db.QueryContext(ctx, selectDownload+` ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list downloads: %w", err)
	}
	tasks := make([]download.Download, 0)
	for rows.Next() {
		task, scanErr := scanDownload(rows)
		if scanErr != nil {
			rows.Close()
			return nil, fmt.Errorf("scan download: %w", scanErr)
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate downloads: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close downloads: %w", err)
	}
	for index := range tasks {
		tasks[index].Segments, err = loadSegments(ctx, r.db, tasks[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return tasks, nil
}

func (r *DownloadRepository) Save(ctx context.Context, task download.Download) error {
	task.Segments = normalizeSegments(task)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin save download: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE downloads SET
		url = ?, final_url = ?, file_name = ?, destination_path = ?, temp_path = ?,
		state = ?, total_bytes = ?, downloaded_bytes = ?, range_supported = ?,
		etag = ?, last_modified = ?, mime_type = ?, created_at = ?, started_at = ?,
		completed_at = ?, last_error = ?, retry_count = ?, connections = ?, bandwidth_limit = ?,
		category_id = ?, queue_id = ?, queue_position = ?, priority = ?, site_profile_id = ? WHERE id = ?`,
		task.URL, task.FinalURL, task.FileName, task.DestinationPath, task.TempPath,
		task.State, task.TotalBytes, task.DownloadedBytes, task.RangeSupported,
		task.ETag, task.LastModified, task.MIMEType, formatTime(task.CreatedAt),
		formatOptionalTime(task.StartedAt), formatOptionalTime(task.CompletedAt),
		task.LastError, task.RetryCount, task.Connections, task.BandwidthLimit,
		task.CategoryID, task.QueueID, task.QueuePosition, task.Priority, task.SiteProfileID, task.ID)
	if err != nil {
		return fmt.Errorf("save download: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read saved download count: %w", err)
	}
	if affected == 0 {
		return download.ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM segments WHERE download_id = ?`, task.ID); err != nil {
		return fmt.Errorf("replace download segments: %w", err)
	}
	if err := insertSegments(ctx, tx, task.Segments); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit save download: %w", err)
	}
	return nil
}

func (r *DownloadRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM downloads WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete download: %w", err)
	}
	return requireAffected(result)
}

const selectDownload = `SELECT
	id, url, final_url, file_name, destination_path, temp_path, state,
	total_bytes, downloaded_bytes, range_supported, etag, last_modified,
	mime_type, created_at, started_at, completed_at, last_error, retry_count, connections, bandwidth_limit,
	category_id, queue_id, queue_position, priority, site_profile_id
	FROM downloads`

type scanner interface{ Scan(...any) error }

func scanDownload(row scanner) (download.Download, error) {
	var task download.Download
	var state, createdAt string
	var startedAt, completedAt sql.NullString
	err := row.Scan(&task.ID, &task.URL, &task.FinalURL, &task.FileName, &task.DestinationPath,
		&task.TempPath, &state, &task.TotalBytes, &task.DownloadedBytes,
		&task.RangeSupported, &task.ETag, &task.LastModified, &task.MIMEType,
		&createdAt, &startedAt, &completedAt, &task.LastError, &task.RetryCount, &task.Connections, &task.BandwidthLimit,
		&task.CategoryID, &task.QueueID, &task.QueuePosition, &task.Priority, &task.SiteProfileID)
	if err != nil {
		return download.Download{}, err
	}
	task.State = download.State(state)
	task.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return download.Download{}, fmt.Errorf("parse created_at: %w", err)
	}
	if startedAt.Valid {
		value, parseErr := time.Parse(time.RFC3339Nano, startedAt.String)
		if parseErr != nil {
			return download.Download{}, fmt.Errorf("parse started_at: %w", parseErr)
		}
		task.StartedAt = &value
	}
	if completedAt.Valid {
		value, parseErr := time.Parse(time.RFC3339Nano, completedAt.String)
		if parseErr != nil {
			return download.Download{}, fmt.Errorf("parse completed_at: %w", parseErr)
		}
		task.CompletedAt = &value
	}
	return task, nil
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func loadSegments(ctx context.Context, db queryer, downloadID string) ([]download.Segment, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, download_id, segment_index, start_byte, end_byte,
		current_byte, state, retry_count, temp_path, last_error
		FROM segments WHERE download_id = ? ORDER BY segment_index`, downloadID)
	if err != nil {
		return nil, fmt.Errorf("load download segments: %w", err)
	}
	defer rows.Close()
	segments := make([]download.Segment, 0)
	for rows.Next() {
		var segment download.Segment
		var state string
		if err := rows.Scan(&segment.ID, &segment.DownloadID, &segment.Index, &segment.StartByte,
			&segment.EndByte, &segment.CurrentByte, &state, &segment.RetryCount,
			&segment.TempPath, &segment.LastError); err != nil {
			return nil, fmt.Errorf("scan download segment: %w", err)
		}
		segment.State = download.SegmentState(state)
		segments = append(segments, segment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate download segments: %w", err)
	}
	if len(segments) == 0 {
		return nil, fmt.Errorf("download %s has no segments", downloadID)
	}
	return segments, nil
}

func normalizeSegments(task download.Download) []download.Segment {
	segments := append([]download.Segment(nil), task.Segments...)
	if len(segments) == 0 {
		end := int64(-1)
		if task.TotalBytes > 0 {
			end = task.TotalBytes - 1
		}
		segments = []download.Segment{{StartByte: 0, EndByte: end, CurrentByte: 0}}
	}
	for index := range segments {
		segment := &segments[index]
		segment.DownloadID = task.ID
		segment.Index = index
		if segment.ID == "" {
			segment.ID = fmt.Sprintf("%s:%d", task.ID, index)
		}
		if segment.State == "" {
			segment.State = download.SegmentPending
		}
		if segment.TempPath == "" {
			segment.TempPath = task.TempPath
		}
	}
	return segments
}

func insertSegments(ctx context.Context, tx *sql.Tx, segments []download.Segment) error {
	for _, segment := range segments {
		if _, err := tx.ExecContext(ctx, `INSERT INTO segments (
			id, download_id, segment_index, start_byte, end_byte, current_byte,
			state, retry_count, temp_path, last_error
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, segmentValues(segment)...); err != nil {
			return fmt.Errorf("save download segment %d: %w", segment.Index, err)
		}
	}
	return nil
}

func downloadValues(task download.Download) []any {
	return []any{task.ID, task.URL, task.FinalURL, task.FileName, task.DestinationPath,
		task.TempPath, task.State, task.TotalBytes, task.DownloadedBytes,
		task.RangeSupported, task.ETag, task.LastModified, task.MIMEType,
		formatTime(task.CreatedAt), formatOptionalTime(task.StartedAt),
		formatOptionalTime(task.CompletedAt), task.LastError, task.RetryCount, task.Connections, task.BandwidthLimit,
		task.CategoryID, task.QueueID, task.QueuePosition, task.Priority, task.SiteProfileID}
}

func segmentValues(segment download.Segment) []any {
	return []any{segment.ID, segment.DownloadID, segment.Index, segment.StartByte,
		segment.EndByte, segment.CurrentByte, segment.State, segment.RetryCount,
		segment.TempPath, segment.LastError}
}

func formatTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }
func formatOptionalTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}
