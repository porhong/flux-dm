package persistence

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/fluxdm/fluxdm/internal/update"
)

type UpdateRepository struct{ db *sql.DB }

func (d *Database) Updates() *UpdateRepository { return &UpdateRepository{db: d.db} }

func (r *UpdateRepository) Load(ctx context.Context) (update.StoredState, error) {
	var state update.StoredState
	var autoDownload int
	err := r.db.QueryRowContext(ctx, `SELECT channel,auto_download,phase,available_version,release_notes_url,installer_path,installer_sha256,downloaded_bytes,total_bytes,last_checked_at,last_error,installation_token,installed_version,installed_at FROM update_state WHERE id=1`).Scan(
		&state.Channel, &autoDownload, &state.Phase, &state.AvailableVersion, &state.ReleaseNotesURL,
		&state.InstallerPath, &state.InstallerSHA256, &state.DownloadedBytes, &state.TotalBytes,
		&state.LastCheckedAt, &state.LastError, &state.InstallationToken, &state.InstalledVersion, &state.InstalledAt,
	)
	if err != nil {
		return state, fmt.Errorf("load update state: %w", err)
	}
	state.AutoDownload = autoDownload != 0
	return state, nil
}

func (r *UpdateRepository) Save(ctx context.Context, state update.StoredState) error {
	autoDownload := 0
	if state.AutoDownload {
		autoDownload = 1
	}
	_, err := r.db.ExecContext(ctx, `UPDATE update_state SET channel=?,auto_download=?,phase=?,available_version=?,release_notes_url=?,installer_path=?,installer_sha256=?,downloaded_bytes=?,total_bytes=?,last_checked_at=?,last_error=?,installation_token=?,installed_version=?,installed_at=?,updated_at=CURRENT_TIMESTAMP WHERE id=1`,
		state.Channel, autoDownload, state.Phase, state.AvailableVersion, state.ReleaseNotesURL,
		state.InstallerPath, state.InstallerSHA256, state.DownloadedBytes, state.TotalBytes,
		state.LastCheckedAt, state.LastError, state.InstallationToken, state.InstalledVersion, state.InstalledAt,
	)
	if err != nil {
		return fmt.Errorf("save update state: %w", err)
	}
	return nil
}
