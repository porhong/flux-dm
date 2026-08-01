package application

import (
	"context"

	"github.com/fluxdm/fluxdm/internal/update"
)

type UpdateDTO struct {
	CurrentVersion   string `json:"currentVersion"`
	Channel          string `json:"channel"`
	AutoDownload     bool   `json:"autoDownload"`
	Phase            string `json:"phase"`
	AvailableVersion string `json:"availableVersion"`
	ReleaseNotesURL  string `json:"releaseNotesUrl"`
	DownloadedBytes  int64  `json:"downloadedBytes"`
	TotalBytes       int64  `json:"totalBytes"`
	LastCheckedAt    string `json:"lastCheckedAt"`
	LastError        string `json:"lastError"`
	Preview          bool   `json:"preview"`
	CanInstall       bool   `json:"canInstall"`
	InstalledVersion string `json:"installedVersion"`
	InstalledAt      string `json:"installedAt"`
}
type UpdatePreferencesInput struct {
	Channel      string `json:"channel"`
	AutoDownload bool   `json:"autoDownload"`
}
type UpdateService struct{ manager *update.Manager }

func NewUpdateService(manager *update.Manager) *UpdateService {
	return &UpdateService{manager: manager}
}
func (s *UpdateService) Close() { s.manager.Close() }
func (s *UpdateService) Load(ctx context.Context) (UpdateDTO, error) {
	value, err := s.manager.Load(ctx)
	return updateDTO(value), err
}
func (s *UpdateService) Status() UpdateDTO { return updateDTO(s.manager.Status()) }
func (s *UpdateService) SavePreferences(ctx context.Context, input UpdatePreferencesInput) (UpdateDTO, error) {
	if input.Channel != update.ChannelStable && input.Channel != update.ChannelPreview {
		return UpdateDTO{}, NewError(ErrInvalidInput, "Choose a supported update channel.", nil)
	}
	value, err := s.manager.SavePreferences(ctx, update.Preferences{Channel: input.Channel, AutoDownload: input.AutoDownload})
	if err != nil {
		return updateDTO(value), NewError(ErrInternal, "Could not save update preferences.", err)
	}
	return updateDTO(value), nil
}
func (s *UpdateService) Check(ctx context.Context) (UpdateDTO, error) {
	value, err := s.manager.Check(ctx, false)
	if err != nil {
		return updateDTO(value), NewError(ErrInternal, "Could not check for application updates. Check your connection and try again.", err)
	}
	return updateDTO(value), nil
}
func (s *UpdateService) Download(ctx context.Context) (UpdateDTO, error) {
	value, err := s.manager.DownloadAvailable(ctx)
	if err != nil {
		return updateDTO(value), NewError(ErrInternal, "Could not download and verify the update. Check your connection and try again.", err)
	}
	return updateDTO(value), nil
}
func (s *UpdateService) Install(ctx context.Context, confirmPreview bool) error {
	return s.manager.Install(ctx, confirmPreview)
}
func updateDTO(value update.Status) UpdateDTO {
	return UpdateDTO{CurrentVersion: value.CurrentVersion, Channel: value.Channel, AutoDownload: value.AutoDownload, Phase: value.Phase, AvailableVersion: value.AvailableVersion, ReleaseNotesURL: value.ReleaseNotesURL, DownloadedBytes: value.DownloadedBytes, TotalBytes: value.TotalBytes, LastCheckedAt: value.LastCheckedAt, LastError: value.LastError, Preview: value.Preview, CanInstall: value.CanInstall, InstalledVersion: value.InstalledVersion, InstalledAt: value.InstalledAt}
}
