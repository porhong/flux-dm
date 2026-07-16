package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fluxdm/fluxdm/internal/download"
	"github.com/fluxdm/fluxdm/internal/organization"
)

type OrganizationRepository struct{ db *sql.DB }

func (d *Database) Organization() *OrganizationRepository { return &OrganizationRepository{db: d.db} }

func (r *OrganizationRepository) ListCategories(ctx context.Context) ([]organization.Category, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, extensions, destination_dir, priority, created_at FROM categories ORDER BY priority DESC, name`)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()
	result := make([]organization.Category, 0)
	for rows.Next() {
		var item organization.Category
		var extensions, created string
		if err := rows.Scan(&item.ID, &item.Name, &extensions, &item.DestinationDir, &item.Priority, &created); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		item.Extensions = organization.NormalizeExtensions(strings.Split(extensions, ","))
		item.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
		if err != nil {
			return nil, fmt.Errorf("parse category timestamp: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *OrganizationRepository) SaveCategory(ctx context.Context, item organization.Category) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO categories(id,name,extensions,destination_dir,priority,created_at) VALUES(?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, extensions=excluded.extensions, destination_dir=excluded.destination_dir, priority=excluded.priority`,
		item.ID, item.Name, strings.Join(organization.NormalizeExtensions(item.Extensions), ","), item.DestinationDir, item.Priority, formatTime(item.CreatedAt))
	if err != nil {
		return fmt.Errorf("save category: %w", err)
	}
	return nil
}

func (r *OrganizationRepository) DeleteCategory(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM categories WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete category: %w", err)
	}
	return requireAffected(result)
}

func (r *OrganizationRepository) ListQueues(ctx context.Context) ([]organization.Queue, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,priority,max_parallel,max_connections,bandwidth_limit,sequential,enabled,created_at FROM download_queues ORDER BY priority DESC, name`)
	if err != nil {
		return nil, fmt.Errorf("list queues: %w", err)
	}
	defer rows.Close()
	result := make([]organization.Queue, 0)
	for rows.Next() {
		var item organization.Queue
		var created string
		if err := rows.Scan(&item.ID, &item.Name, &item.Priority, &item.MaxParallel, &item.MaxConnections, &item.BandwidthLimit, &item.Sequential, &item.Enabled, &created); err != nil {
			return nil, fmt.Errorf("scan queue: %w", err)
		}
		item.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
		if err != nil {
			return nil, fmt.Errorf("parse queue timestamp: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *OrganizationRepository) GetQueue(ctx context.Context, id string) (organization.Queue, error) {
	var item organization.Queue
	var created string
	err := r.db.QueryRowContext(ctx, `SELECT id,name,priority,max_parallel,max_connections,bandwidth_limit,sequential,enabled,created_at FROM download_queues WHERE id=?`, id).
		Scan(&item.ID, &item.Name, &item.Priority, &item.MaxParallel, &item.MaxConnections, &item.BandwidthLimit, &item.Sequential, &item.Enabled, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return item, download.ErrNotFound
	}
	if err != nil {
		return item, fmt.Errorf("get queue: %w", err)
	}
	item.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	return item, err
}

func (r *OrganizationRepository) SaveQueue(ctx context.Context, item organization.Queue) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO download_queues(id,name,priority,max_parallel,max_connections,bandwidth_limit,sequential,enabled,created_at) VALUES(?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name,priority=excluded.priority,max_parallel=excluded.max_parallel,max_connections=excluded.max_connections,bandwidth_limit=excluded.bandwidth_limit,sequential=excluded.sequential,enabled=excluded.enabled`,
		item.ID, item.Name, item.Priority, item.MaxParallel, item.MaxConnections, item.BandwidthLimit, item.Sequential, item.Enabled, formatTime(item.CreatedAt))
	if err != nil {
		return fmt.Errorf("save queue: %w", err)
	}
	return nil
}

func (r *OrganizationRepository) DeleteQueue(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM download_queues WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete queue: %w", err)
	}
	return requireAffected(result)
}

func requireAffected(result sql.Result) error {
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return download.ErrNotFound
	}
	return nil
}
