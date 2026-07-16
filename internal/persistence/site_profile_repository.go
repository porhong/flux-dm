package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/fluxdm/fluxdm/internal/download"
	"github.com/fluxdm/fluxdm/internal/siteprofile"
	"time"
)

type SiteProfileRepository struct{ db *sql.DB }

func (d *Database) SiteProfiles() *SiteProfileRepository { return &SiteProfileRepository{db: d.db} }
func (r *SiteProfileRepository) List(ctx context.Context) ([]siteprofile.Record, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,host_pattern,auth_type,proxy_url,encrypted_secrets,created_at FROM site_profiles ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list site profiles: %w", err)
	}
	defer rows.Close()
	result := make([]siteprofile.Record, 0)
	for rows.Next() {
		record, err := scanSiteProfile(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, record)
	}
	return result, rows.Err()
}
func (r *SiteProfileRepository) Get(ctx context.Context, id string) (siteprofile.Record, error) {
	record, err := scanSiteProfile(r.db.QueryRowContext(ctx, `SELECT id,name,host_pattern,auth_type,proxy_url,encrypted_secrets,created_at FROM site_profiles WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return record, download.ErrNotFound
	}
	return record, err
}
func (r *SiteProfileRepository) Save(ctx context.Context, record siteprofile.Record) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO site_profiles(id,name,host_pattern,auth_type,proxy_url,encrypted_secrets,created_at)VALUES(?,?,?,?,?,?,?) ON CONFLICT(id)DO UPDATE SET name=excluded.name,host_pattern=excluded.host_pattern,auth_type=excluded.auth_type,proxy_url=excluded.proxy_url,encrypted_secrets=excluded.encrypted_secrets`, record.Profile.ID, record.Profile.Name, record.Profile.HostPattern, record.Profile.AuthType, record.Profile.ProxyURL, record.EncryptedSecrets, formatTime(record.Profile.CreatedAt))
	if err != nil {
		return fmt.Errorf("save site profile: %w", err)
	}
	return nil
}
func (r *SiteProfileRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM site_profiles WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete site profile: %w", err)
	}
	return requireAffected(result)
}
func (r *SiteProfileRepository) SaveDownloadSecret(ctx context.Context, id string, ciphertext []byte) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO download_secrets(download_id,encrypted_secrets,created_at)VALUES(?,?,?)ON CONFLICT(download_id)DO UPDATE SET encrypted_secrets=excluded.encrypted_secrets,created_at=excluded.created_at`, id, ciphertext, formatTime(time.Now().UTC()))
	return err
}
func (r *SiteProfileRepository) GetDownloadSecret(ctx context.Context, id string) ([]byte, error) {
	var value []byte
	err := r.db.QueryRowContext(ctx, `SELECT encrypted_secrets FROM download_secrets WHERE download_id=?`, id).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return value, err
}
func (r *SiteProfileRepository) DeleteDownloadSecret(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM download_secrets WHERE download_id=?`, id)
	return err
}

type siteProfileScanner interface{ Scan(...any) error }

func scanSiteProfile(row siteProfileScanner) (siteprofile.Record, error) {
	var record siteprofile.Record
	var created string
	err := row.Scan(&record.Profile.ID, &record.Profile.Name, &record.Profile.HostPattern, &record.Profile.AuthType, &record.Profile.ProxyURL, &record.EncryptedSecrets, &created)
	if err != nil {
		return record, err
	}
	record.Profile.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	return record, err
}
