package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/fluxdm/fluxdm/internal/application"
	"github.com/fluxdm/fluxdm/internal/browserintegration"
	"github.com/fluxdm/fluxdm/internal/download"
	"github.com/fluxdm/fluxdm/internal/events"
	fluxlog "github.com/fluxdm/fluxdm/internal/logging"
	"github.com/fluxdm/fluxdm/internal/organization"
	"github.com/fluxdm/fluxdm/internal/persistence"
	platformwindows "github.com/fluxdm/fluxdm/internal/platform/windows"
	"github.com/fluxdm/fluxdm/internal/scheduler"
	"github.com/fluxdm/fluxdm/internal/secrets"
	"github.com/fluxdm/fluxdm/internal/siteprofile"
	"github.com/fluxdm/fluxdm/internal/transport"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const appReadyEvent = "app:ready"

// App is the thin Wails adapter for FluxDM's application services.
type App struct {
	ctx           context.Context
	paths         application.Paths
	bus           *events.Bus
	logger        *fluxlog.Logger
	database      *persistence.Database
	downloads     *application.DownloadService
	organization  *application.OrganizationService
	schedules     *application.SchedulerService
	browserBridge *browserintegration.Server
	siteProfiles  *application.SiteProfileService
	forceQuit     atomic.Bool
	trayStarted   atomic.Bool
}

func NewApp(paths application.Paths, logger *fluxlog.Logger) *App {
	return &App{
		paths:  paths,
		bus:    events.NewBus(),
		logger: logger,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.startTray()
	if executable, err := os.Executable(); err == nil {
		if notifyErr := platformwindows.ConfigureNotifications(executable, ""); notifyErr != nil {
			a.logger.Error("notification setup failed", map[string]any{"error": notifyErr.Error()})
		}
	}

	database, recovery, err := persistence.OpenRecovering(ctx, filepath.Join(a.paths.DataDir, "fluxdm.db"))
	if err != nil {
		a.logger.Error("database initialization failed", map[string]any{"error": err.Error()})
		runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
			Type:    runtime.ErrorDialog,
			Title:   "FluxDM could not start",
			Message: "The local database could not be initialized.",
		})
		return
	}
	a.database = database
	if recovery.BackupPath != "" {
		a.logger.Error("database corruption recovered", map[string]any{"backup_created": true})
	}

	a.bus.Subscribe(events.AppReady, func(event events.Event) {
		runtime.EventsEmit(ctx, appReadyEvent, application.ReadyEvent{
			Name:    "FluxDM",
			Version: application.Version,
			Message: event.Message,
		})
	})
	a.bus.Subscribe(events.DownloadProgress, func(event events.Event) {
		runtime.EventsEmit(ctx, "download:progress", event.Data)
	})
	a.bus.Subscribe(events.DownloadUpdated, func(event events.Event) {
		runtime.EventsEmit(ctx, "download:updated", event.Data)
		if dto, ok := event.Data.(application.DownloadDTO); ok && dto.State == string(download.StateCompleted) {
			if err := platformwindows.NotifyDownloadComplete(dto.FileName); err != nil {
				a.logger.Error("download notification failed", map[string]any{"error": err.Error()})
			}
		}
	})
	httpClient := transport.NewHTTPClient()
	organizationRepository := database.Organization()
	a.organization = application.NewOrganizationService(organizationRepository, database.Downloads())
	a.siteProfiles = application.NewSiteProfileService(database.SiteProfiles(), secrets.DPAPI{})
	a.downloads = application.NewDownloadService(
		ctx,
		database.Downloads(),
		download.NewProber(httpClient),
		download.NewEngine(httpClient),
		a.bus,
		organizationRepository,
	)
	a.downloads.SetRequestProfileResolver(a.siteProfiles)
	if err := a.downloads.Recover(ctx); err != nil {
		a.logger.Error("download recovery failed", map[string]any{"error": err.Error()})
	}
	if bridge, bridgeErr := browserintegration.StartServer(a.paths.DataDir, a.acceptBrowserRequest); bridgeErr != nil {
		a.logger.Error("browser integration startup failed", map[string]any{"error": bridgeErr.Error()})
	} else {
		a.browserBridge = bridge
	}
	a.schedules = application.NewSchedulerService(ctx, database.Scheduler(), a, organizationRepository)
	a.bus.Publish(events.Event{Type: events.AppReady, Message: "Backend services are ready"})
	a.logger.Info("application started", map[string]any{"version": application.Version})
}

func (a *App) shutdown(_ context.Context) {
	stopTray()
	if a.schedules != nil {
		a.schedules.Close()
	}
	if a.browserBridge != nil {
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = a.browserBridge.Close(closeCtx)
		cancel()
	}
	if a.downloads != nil {
		a.downloads.Close()
	}
	if a.database == nil {
		return
	}
	if err := a.database.Close(); err != nil {
		a.logger.Error("database shutdown failed", map[string]any{"error": err.Error()})
	}
}

func (a *App) acceptBrowserRequest(ctx context.Context, message browserintegration.Request) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	directory := filepath.Join(home, "Downloads")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	created, err := a.downloads.CreateWithCookies(ctx, application.CreateDownloadInput{URL: message.URL, DestinationDir: directory, FileName: message.SuggestedFilename, Connections: 4}, message.Cookies)
	if err != nil {
		return err
	}
	return a.downloads.Start(ctx, created.ID)
}

func (a *App) ExecuteSchedule(ctx context.Context, item scheduler.Schedule) error {
	switch item.Action {
	case scheduler.ActionStartQueue:
		if err := a.organization.SetQueueEnabled(ctx, item.QueueID, true); err != nil {
			return err
		}
		return a.downloads.StartQueue(ctx, item.QueueID, false)
	case scheduler.ActionStopQueue:
		if err := a.organization.SetQueueEnabled(ctx, item.QueueID, false); err != nil {
			return err
		}
		return a.downloads.StopQueue(ctx, item.QueueID)
	case scheduler.ActionSpeedProfile:
		return a.downloads.SetGlobalBandwidthLimit(item.SpeedLimit)
	case scheduler.ActionRetryFailed:
		return a.downloads.RetryFailed(ctx, item.QueueID)
	default:
		return errors.New("unsupported schedule action")
	}
}

func (a *App) ExecutePostAction(ctx context.Context, item scheduler.Schedule) error {
	if item.Action == scheduler.ActionStartQueue || item.Action == scheduler.ActionRetryFailed {
		if err := a.downloads.WaitForIdle(ctx, item.QueueID); err != nil {
			return err
		}
	}
	switch item.PostAction {
	case scheduler.PostNone:
		return nil
	case scheduler.PostExit:
		a.forceQuit.Store(true)
		runtime.Quit(a.ctx)
		return nil
	case scheduler.PostSleep:
		return platformwindows.Sleep()
	case scheduler.PostHibernate:
		return platformwindows.Hibernate()
	case scheduler.PostShutdown:
		return platformwindows.Shutdown()
	default:
		return errors.New("unsupported post action")
	}
}

func (a *App) beforeClose(ctx context.Context) bool {
	if a.forceQuit.Load() {
		return false
	}
	runtime.WindowHide(ctx)
	return true
}

// showWindow restores the tray-hidden window after a second FluxDM launch.
func (a *App) showWindow() {
	if a.ctx != nil {
		runtime.WindowShow(a.ctx)
	}
}

func (a *App) ProbeURL(rawURL string) (application.ProbeDTO, error) {
	if a.downloads == nil {
		return application.ProbeDTO{}, application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.downloads.Probe(a.ctx, rawURL)
}

func (a *App) CreateDownload(input application.CreateDownloadInput) (application.DownloadDTO, error) {
	if a.downloads == nil {
		return application.DownloadDTO{}, application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.downloads.Create(a.ctx, input)
}

func (a *App) StartDownload(id string) error {
	if a.downloads == nil {
		return application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.downloads.Start(a.ctx, id)
}

func (a *App) CancelDownload(id string) error {
	if a.downloads == nil {
		return application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.downloads.Cancel(a.ctx, id)
}

func (a *App) PauseDownload(id string) error {
	if a.downloads == nil {
		return application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.downloads.Pause(a.ctx, id)
}

func (a *App) ResumeDownload(id string) error {
	if a.downloads == nil {
		return application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.downloads.Resume(a.ctx, id)
}

func (a *App) RestartDownload(id string) error {
	if a.downloads == nil {
		return application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.downloads.Restart(a.ctx, id)
}

func (a *App) SetGlobalBandwidthLimit(limit int64) error {
	if a.downloads == nil {
		return application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.downloads.SetGlobalBandwidthLimit(limit)
}

func (a *App) SetDownloadBandwidthLimit(id string, limit int64) error {
	if a.downloads == nil {
		return application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.downloads.SetDownloadBandwidthLimit(a.ctx, id, limit)
}

func (a *App) ListDownloads() ([]application.DownloadDTO, error) {
	if a.downloads == nil {
		return nil, application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.downloads.List(a.ctx)
}

func (a *App) GetDownload(id string) (application.DownloadDTO, error) {
	if a.downloads == nil {
		return application.DownloadDTO{}, application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.downloads.Get(a.ctx, id)
}

func (a *App) ListCategories() ([]organization.Category, error) {
	if a.organization == nil {
		return nil, application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.organization.ListCategories(a.ctx)
}

func (a *App) SaveCategory(input application.SaveCategoryInput) (organization.Category, error) {
	if a.organization == nil {
		return organization.Category{}, application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.organization.SaveCategory(a.ctx, input)
}

func (a *App) DeleteCategory(id string) error {
	if a.organization == nil {
		return application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.organization.DeleteCategory(a.ctx, id)
}

func (a *App) ListQueues() ([]organization.Queue, error) {
	if a.organization == nil {
		return nil, application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.organization.ListQueues(a.ctx)
}

func (a *App) SaveQueue(input application.SaveQueueInput) (organization.Queue, error) {
	if a.organization == nil {
		return organization.Queue{}, application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.organization.SaveQueue(a.ctx, input)
}

func (a *App) DeleteQueue(id string) error {
	if a.organization == nil {
		return application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.organization.DeleteQueue(a.ctx, id)
}

func (a *App) AssignDownloads(input application.AssignDownloadsInput) error {
	if a.organization == nil {
		return application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.organization.AssignDownloads(a.ctx, input)
}

func (a *App) ListSchedules() ([]scheduler.Schedule, error) {
	if a.schedules == nil {
		return nil, application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.schedules.List(a.ctx)
}

func (a *App) SaveSchedule(input application.SaveScheduleInput) (scheduler.Schedule, error) {
	if a.schedules == nil {
		return scheduler.Schedule{}, application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.schedules.Save(a.ctx, input)
}

func (a *App) DeleteSchedule(id string) error {
	if a.schedules == nil {
		return application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.schedules.Delete(a.ctx, id)
}

func (a *App) ListScheduleHistory(limit int) ([]scheduler.History, error) {
	if a.schedules == nil {
		return nil, application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.schedules.History(a.ctx, limit)
}

func (a *App) ListSiteProfiles() ([]siteprofile.Profile, error) {
	if a.siteProfiles == nil {
		return nil, application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.siteProfiles.List(a.ctx)
}
func (a *App) SaveSiteProfile(input application.SaveSiteProfileInput) (siteprofile.Profile, error) {
	if a.siteProfiles == nil {
		return siteprofile.Profile{}, application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.siteProfiles.Save(a.ctx, input)
}
func (a *App) DeleteSiteProfile(id string) error {
	if a.siteProfiles == nil {
		return application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.siteProfiles.Delete(a.ctx, id)
}
func (a *App) ClearSiteProfileSecrets(id string) error {
	if a.siteProfiles == nil {
		return application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return a.siteProfiles.ClearSecrets(a.ctx, id)
}

func (a *App) ClearPrivateData() error {
	if a.database == nil {
		return application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	if err := a.database.ClearPrivateData(a.ctx); err != nil {
		return application.NewError(application.ErrInternal, "Could not clear private data.", err)
	}
	if err := a.logger.Clear(); err != nil {
		return application.NewError(application.ErrInternal, "Private data was cleared, but the log could not be cleared.", err)
	}
	return nil
}

func (a *App) SelectDestinationDirectory() (string, error) {
	if a.ctx == nil {
		return "", application.NewError(application.ErrUnavailable, "Backend is not ready.", nil)
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Choose download folder",
	})
}

// DefaultDownloadDirectory returns the user's standard Downloads folder for
// pre-populating the download confirmation dialog.
func (a *App) DefaultDownloadDirectory() (string, error) {
	directory, err := application.DefaultDownloadDirectory()
	if err != nil {
		return "", application.NewError(application.ErrInternal, "Could not prepare the default Downloads folder.", err)
	}
	return directory, nil
}

// HealthCheck confirms that the backend and persistence layer are available.
func (a *App) HealthCheck() (application.HealthStatus, error) {
	if a.database == nil {
		return application.HealthStatus{}, application.NewError(
			application.ErrUnavailable,
			"backend is not ready",
			errors.New("database is not initialized"),
		)
	}
	if err := a.database.Ping(a.ctx); err != nil {
		return application.HealthStatus{}, application.NewError(
			application.ErrUnavailable,
			"database health check failed",
			err,
		)
	}
	a.bus.Publish(events.Event{Type: events.AppReady, Message: "Health check completed"})
	return application.NewHealthStatus(), nil
}
