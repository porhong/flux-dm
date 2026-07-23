package application

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fluxdm/fluxdm/internal/download"
	"github.com/fluxdm/fluxdm/internal/events"
	fluxfs "github.com/fluxdm/fluxdm/internal/filesystem"
	"github.com/fluxdm/fluxdm/internal/organization"
	"github.com/fluxdm/fluxdm/internal/security"
)

const (
	defaultDownloadWorkers = 16
	maxQueuedDownloads     = 64
	checkpointInterval     = time.Second
)

var ErrRemoteChanged = errors.New("remote resource changed")

type CreateDownloadInput struct {
	URL               string `json:"url"`
	DestinationDir    string `json:"destinationDir"`
	FileName          string `json:"fileName"`
	Connections       int    `json:"connections"`
	BandwidthLimit    int64  `json:"bandwidthLimit"`
	CategoryID        string `json:"categoryId"`
	QueueID           string `json:"queueId"`
	Priority          int    `json:"priority"`
	SiteProfileID     string `json:"siteProfileId"`
	ConfirmExecutable bool   `json:"confirmExecutable"`
}

type ProbeDTO struct {
	URL               string `json:"url"`
	FinalURL          string `json:"finalUrl"`
	FileName          string `json:"fileName"`
	TotalBytes        int64  `json:"totalBytes"`
	MIMEType          string `json:"mimeType"`
	ETag              string `json:"etag"`
	LastModified      string `json:"lastModified"`
	RangeSupported    bool   `json:"rangeSupported"`
	ExecutableWarning bool   `json:"executableWarning"`
}

type DownloadDTO struct {
	ID              string  `json:"id"`
	URL             string  `json:"url"`
	FinalURL        string  `json:"finalUrl"`
	FileName        string  `json:"fileName"`
	DestinationPath string  `json:"destinationPath"`
	TempPath        string  `json:"tempPath"`
	State           string  `json:"state"`
	TotalBytes      int64   `json:"totalBytes"`
	DownloadedBytes int64   `json:"downloadedBytes"`
	RangeSupported  bool    `json:"rangeSupported"`
	RestartRequired bool    `json:"restartRequired"`
	MIMEType        string  `json:"mimeType"`
	CreatedAt       string  `json:"createdAt"`
	StartedAt       *string `json:"startedAt"`
	CompletedAt     *string `json:"completedAt"`
	LastError       string  `json:"lastError"`
	RetryCount      int     `json:"retryCount"`
	Connections     int     `json:"connections"`
	SegmentCount    int     `json:"segmentCount"`
	BandwidthLimit  int64   `json:"bandwidthLimit"`
	CategoryID      string  `json:"categoryId"`
	QueueID         string  `json:"queueId"`
	QueuePosition   int64   `json:"queuePosition"`
	Priority        int     `json:"priority"`
	SiteProfileID   string  `json:"siteProfileId"`
}

type jobAction uint8

const (
	actionStart jobAction = iota
	actionResume
)

type downloadJob struct {
	id          string
	action      jobAction
	queueID     string
	priority    int
	maxParallel int
}

type stopIntent int32

const (
	intentNone stopIntent = iota
	intentPause
	intentCancel
)

type runControl struct {
	mu       sync.Mutex
	cancel   context.CancelFunc
	intent   atomic.Int32
	finished atomic.Bool
}

type DownloadService struct {
	repository      download.Repository
	organization    organization.Repository
	profileResolver RequestProfileResolver
	prober          *download.Prober
	engine          *download.Engine
	bus             *events.Bus
	ctx             context.Context
	cancel          context.CancelFunc
	wake            chan struct{}
	pending         []downloadJob
	queueRunning    map[string]int
	wg              sync.WaitGroup
	mu              sync.Mutex
	queued          map[string]struct{}
	running         map[string]*runControl
}

func NewDownloadService(parent context.Context, repository download.Repository, prober *download.Prober, engine *download.Engine, bus *events.Bus, organizations ...organization.Repository) *DownloadService {
	ctx, cancel := context.WithCancel(parent)
	service := &DownloadService{
		repository:   repository,
		prober:       prober,
		engine:       engine,
		bus:          bus,
		ctx:          ctx,
		cancel:       cancel,
		wake:         make(chan struct{}, 1),
		pending:      make([]downloadJob, 0, maxQueuedDownloads),
		queueRunning: make(map[string]int),
		queued:       make(map[string]struct{}),
		running:      make(map[string]*runControl),
	}
	if len(organizations) > 0 {
		service.organization = organizations[0]
	}
	for range defaultDownloadWorkers {
		service.wg.Add(1)
		go service.worker()
	}
	return service
}

func (s *DownloadService) SetRequestProfileResolver(resolver RequestProfileResolver) {
	s.profileResolver = resolver
}

func (s *DownloadService) Close() {
	s.cancel()
	s.wg.Wait()
}

func (s *DownloadService) Recover(ctx context.Context) error {
	tasks, err := s.repository.List(ctx)
	if err != nil {
		return err
	}
	for index := range tasks {
		task := &tasks[index]
		if !isRecoverableState(task.State) {
			continue
		}
		if err := s.recoverTask(ctx, task); err != nil {
			return err
		}
	}
	return nil
}

func (s *DownloadService) Probe(ctx context.Context, rawURL string) (ProbeDTO, error) {
	options := download.RequestOptions{}
	if s.profileResolver != nil {
		_, resolved, resolveErr := s.profileResolver.Resolve(ctx, rawURL, "", "")
		if resolveErr != nil {
			return ProbeDTO{}, NewError(ErrInvalidInput, "Could not apply the site profile.", resolveErr)
		}
		options = resolved
	}
	result, err := s.prober.ProbeWithOptions(ctx, rawURL, options)
	if err != nil {
		return ProbeDTO{}, NewError(ErrInvalidInput, probeErrorMessage(err), err)
	}
	return probeToDTO(result), nil
}

func (s *DownloadService) Create(ctx context.Context, input CreateDownloadInput) (DownloadDTO, error) {
	return s.create(ctx, input, "")
}

func (s *DownloadService) CreateWithCookies(ctx context.Context, input CreateDownloadInput, cookies string) (DownloadDTO, error) {
	return s.create(ctx, input, cookies)
}

func (s *DownloadService) create(ctx context.Context, input CreateDownloadInput, cookies string) (DownloadDTO, error) {
	parsed, err := security.ValidateHTTPURL(strings.TrimSpace(input.URL))
	if err != nil {
		return DownloadDTO{}, NewError(ErrInvalidInput, "Enter a valid HTTP or HTTPS URL.", err)
	}
	fileName := strings.TrimSpace(input.FileName)
	if fileName == "" {
		fileName = fileNameFromURL(parsed)
	}
	if fluxfs.IsExecutableLike(fileName) && !input.ConfirmExecutable {
		return DownloadDTO{}, NewError(ErrInvalidInput, "This file type can run code. Confirm the executable download before adding it.", nil)
	}
	connections := input.Connections
	if connections == 0 {
		connections = 4
	}
	if !download.ValidConnectionCount(connections) {
		return DownloadDTO{}, NewError(ErrInvalidInput, "Choose 1, 2, 4, 8, or 16 connections.", nil)
	}
	if input.BandwidthLimit < 0 {
		return DownloadDTO{}, NewError(ErrInvalidInput, "Bandwidth limit cannot be negative.", nil)
	}
	if input.Priority < -1000 || input.Priority > 1000 {
		return DownloadDTO{}, NewError(ErrInvalidInput, "Priority must be between -1000 and 1000.", nil)
	}
	categoryID := strings.TrimSpace(input.CategoryID)
	queueID := strings.TrimSpace(input.QueueID)
	siteProfileID := strings.TrimSpace(input.SiteProfileID)
	if siteProfileID != "" {
		if _, err := validateID(siteProfileID); err != nil {
			return DownloadDTO{}, NewError(ErrInvalidInput, "Invalid site profile identifier.", err)
		}
	}
	if s.profileResolver != nil {
		resolvedID, _, resolveErr := s.profileResolver.Resolve(ctx, parsed.String(), siteProfileID, "")
		if resolveErr != nil {
			return DownloadDTO{}, NewError(ErrInvalidInput, "Could not apply the site profile.", resolveErr)
		}
		siteProfileID = resolvedID
	}
	if s.organization != nil {
		categories, listErr := s.organization.ListCategories(ctx)
		if listErr != nil {
			return DownloadDTO{}, NewError(ErrInternal, "Could not read category rules.", listErr)
		}
		if categoryID == "" {
			if match := organization.MatchCategory(fileName, categories); match != nil {
				categoryID = match.ID
				if strings.TrimSpace(input.DestinationDir) == "" {
					input.DestinationDir = match.DestinationDir
				}
			}
		}
		if categoryID != "" {
			found := false
			for _, category := range categories {
				if category.ID == categoryID {
					found = true
					break
				}
			}
			if !found {
				return DownloadDTO{}, NewError(ErrInvalidInput, "Choose an existing category.", nil)
			}
		}
		if queueID != "" {
			queue, queueErr := s.organization.GetQueue(ctx, queueID)
			if queueErr != nil {
				return DownloadDTO{}, NewError(ErrInvalidInput, "Choose an existing queue.", queueErr)
			}
			if connections > queue.MaxConnections {
				connections = queue.MaxConnections
			}
		}
	}
	directory, err := fluxfs.ValidateDestinationDirectory(strings.TrimSpace(input.DestinationDir))
	if err != nil {
		return DownloadDTO{}, NewError(ErrInvalidInput, "Choose an existing destination folder.", err)
	}
	finalPath, tempPath, fileName, err := fluxfs.ReserveDestination(directory, fileName)
	if err != nil {
		return DownloadDTO{}, NewError(ErrInvalidInput, "FluxDM could not use that destination.", err)
	}
	id := newID()
	if s.profileResolver != nil && cookies != "" {
		if err := s.profileResolver.SaveDownloadCookies(ctx, id, cookies); err != nil {
			_ = os.Remove(tempPath)
			return DownloadDTO{}, NewError(ErrInvalidInput, "Could not protect browser session cookies.", err)
		}
	}
	task := download.Download{
		ID: id, URL: parsed.String(), FileName: fileName,
		DestinationPath: finalPath, TempPath: tempPath, State: download.StateQueued,
		TotalBytes: -1, CreatedAt: time.Now().UTC(), Connections: connections, BandwidthLimit: input.BandwidthLimit,
		CategoryID: categoryID, QueueID: queueID, QueuePosition: time.Now().UTC().UnixNano(), Priority: input.Priority,
		SiteProfileID: siteProfileID,
		Segments: []download.Segment{{
			ID: id + ":0", DownloadID: id, Index: 0, StartByte: 0, EndByte: -1,
			CurrentByte: 0, State: download.SegmentPending, TempPath: tempPath,
		}},
	}
	if err := s.repository.Create(ctx, task); err != nil {
		if s.profileResolver != nil {
			_ = s.profileResolver.ClearDownloadSecrets(ctx, id)
		}
		_ = os.Remove(tempPath)
		return DownloadDTO{}, NewError(ErrInternal, "Could not save the download.", err)
	}
	dto := downloadToDTO(task)
	s.publishUpdated(dto)
	return dto, nil
}

func (s *DownloadService) Start(ctx context.Context, id string) error {
	id, err := validateID(id)
	if err != nil {
		return NewError(ErrInvalidInput, "Invalid download identifier.", err)
	}
	task, err := s.repository.Get(ctx, id)
	if err != nil {
		return repositoryError("start", err)
	}
	action := actionStart
	switch task.State {
	case download.StateQueued:
	case download.StateFailed:
		if err := task.Transition(download.StateRetrying); err != nil {
			return NewError(ErrInternal, "Could not retry the download.", err)
		}
		task.RetryCount++
		for index := range task.Segments {
			task.Segments[index].RetryCount++
			task.Segments[index].LastError = ""
		}
		task.LastError = ""
		if err := s.repository.Save(ctx, task); err != nil {
			return repositoryError("retry", err)
		}
		s.publishUpdated(downloadToDTO(task))
		action = actionResume
	case download.StateCancelled:
		return s.Restart(ctx, id)
	default:
		return NewError(ErrInvalidInput, "Only queued, failed, or cancelled downloads can be started.", nil)
	}
	if err := s.enqueue(downloadJob{id: id, action: action}); err != nil {
		if task.State == download.StateRetrying {
			_ = task.Transition(download.StateFailed)
			_ = s.repository.Save(ctx, task)
		}
		return err
	}
	return nil
}

func (s *DownloadService) Pause(ctx context.Context, id string) error {
	id, err := validateID(id)
	if err != nil {
		return NewError(ErrInvalidInput, "Invalid download identifier.", err)
	}
	control := s.runningControl(id)
	if control == nil {
		task, getErr := s.repository.Get(ctx, id)
		if getErr == nil && task.State == download.StatePaused {
			return nil
		}
		return NewError(ErrInvalidInput, "Only an active download can be paused.", nil)
	}
	control.mu.Lock()
	defer control.mu.Unlock()
	if control.finished.Load() {
		return nil
	}
	control.intent.Store(int32(intentPause))
	task, err := s.repository.Get(ctx, id)
	if err != nil {
		control.intent.Store(int32(intentNone))
		return repositoryError("pause", err)
	}
	if task.State != download.StateDownloading {
		control.intent.Store(int32(intentNone))
		return NewError(ErrInvalidInput, "The download is not ready to pause.", nil)
	}
	if err := task.Transition(download.StatePausing); err != nil {
		control.intent.Store(int32(intentNone))
		return NewError(ErrInternal, "Could not pause the download.", err)
	}
	if err := s.repository.Save(ctx, task); err != nil {
		control.intent.Store(int32(intentNone))
		return repositoryError("pause", err)
	}
	s.publishUpdated(downloadToDTO(task))
	control.cancel()
	return nil
}

func (s *DownloadService) Resume(ctx context.Context, id string) error {
	id, err := validateID(id)
	if err != nil {
		return NewError(ErrInvalidInput, "Invalid download identifier.", err)
	}
	task, err := s.repository.Get(ctx, id)
	if err != nil {
		return repositoryError("resume", err)
	}
	if task.State != download.StatePaused {
		return NewError(ErrInvalidInput, "Only a paused download can be resumed.", nil)
	}
	return s.enqueue(downloadJob{id: id, action: actionResume})
}

func (s *DownloadService) Restart(ctx context.Context, id string) error {
	id, err := validateID(id)
	if err != nil {
		return NewError(ErrInvalidInput, "Invalid download identifier.", err)
	}
	task, err := s.repository.Get(ctx, id)
	if err != nil {
		return repositoryError("restart", err)
	}
	if task.State != download.StatePaused && task.State != download.StateFailed && task.State != download.StateCancelled {
		return NewError(ErrInvalidInput, "Only paused, failed, or cancelled downloads can be restarted.", nil)
	}
	if err := task.Transition(download.StateQueued); err != nil {
		return NewError(ErrInternal, "Could not restart the download.", err)
	}
	reservation, err := os.OpenFile(task.TempPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return NewError(ErrInternal, "Could not reset the temporary file.", err)
	}
	if err := reservation.Close(); err != nil {
		return NewError(ErrInternal, "Could not reset the temporary file.", err)
	}
	task.FinalURL = ""
	task.TotalBytes = -1
	task.DownloadedBytes = 0
	task.RangeSupported = false
	task.ETag = ""
	task.LastModified = ""
	task.LastError = ""
	task.Segments = []download.Segment{{
		ID: task.ID + ":0", DownloadID: task.ID, Index: 0, StartByte: 0, EndByte: -1,
		CurrentByte: 0, State: download.SegmentPending, TempPath: task.TempPath,
	}}
	if err := s.repository.Save(ctx, task); err != nil {
		return repositoryError("restart", err)
	}
	s.publishUpdated(downloadToDTO(task))
	return s.enqueue(downloadJob{id: id, action: actionStart})
}

func (s *DownloadService) Cancel(ctx context.Context, id string) error {
	id, err := validateID(id)
	if err != nil {
		return NewError(ErrInvalidInput, "Invalid download identifier.", err)
	}
	if control := s.runningControl(id); control != nil {
		control.mu.Lock()
		control.intent.Store(int32(intentCancel))
		control.cancel()
		control.mu.Unlock()
		return nil
	}
	task, err := s.repository.Get(ctx, id)
	if err != nil {
		return repositoryError("cancel", err)
	}
	if task.State == download.StateCancelled {
		return nil
	}
	if task.State != download.StateQueued && task.State != download.StatePaused {
		return NewError(ErrInvalidInput, "Only queued, paused, or active downloads can be cancelled.", nil)
	}
	if err := task.Transition(download.StateCancelled); err != nil {
		return NewError(ErrInternal, "Could not cancel the download.", err)
	}
	task.DownloadedBytes = 0
	task.Segments = []download.Segment{{
		ID: task.ID + ":0", DownloadID: task.ID, Index: 0, StartByte: 0, EndByte: -1,
		CurrentByte: 0, State: download.SegmentPending, TempPath: task.TempPath,
	}}
	if err := s.repository.Save(ctx, task); err != nil {
		return repositoryError("cancel", err)
	}
	s.mu.Lock()
	delete(s.queued, id)
	s.mu.Unlock()
	_ = os.Remove(task.TempPath)
	if s.profileResolver != nil {
		_ = s.profileResolver.ClearDownloadSecrets(ctx, id)
	}
	s.publishUpdated(downloadToDTO(task))
	return nil
}

func (s *DownloadService) List(ctx context.Context) ([]DownloadDTO, error) {
	tasks, err := s.repository.List(ctx)
	if err != nil {
		return nil, NewError(ErrInternal, "Could not list downloads.", err)
	}
	result := make([]DownloadDTO, len(tasks))
	for index := range tasks {
		result[index] = downloadToDTO(tasks[index])
	}
	return result, nil
}

func (s *DownloadService) Get(ctx context.Context, id string) (DownloadDTO, error) {
	id, err := validateID(id)
	if err != nil {
		return DownloadDTO{}, NewError(ErrInvalidInput, "Invalid download identifier.", err)
	}
	task, err := s.repository.Get(ctx, id)
	if err != nil {
		return DownloadDTO{}, repositoryError("get", err)
	}
	return downloadToDTO(task), nil
}

// RemoveRecord removes a completed download from FluxDM's history while
// preserving the downloaded file on disk.
func (s *DownloadService) RemoveRecord(ctx context.Context, id string) error {
	task, err := s.completedDownload(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, task.ID); err != nil {
		return repositoryError("remove", err)
	}
	s.clearDownloadSecrets(ctx, task.ID)
	return nil
}

// DeleteCompletedFile removes a completed download's file and then removes
// its history record. The record remains available when the file cannot be
// deleted so the user can choose to keep the record or retry the operation.
func (s *DownloadService) DeleteCompletedFile(ctx context.Context, id string) error {
	task, err := s.completedDownload(ctx, id)
	if err != nil {
		return err
	}
	filePath, err := completedFilePath(task)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewError(ErrInvalidInput, "The completed file is already missing. Remove the record instead.", err)
		}
		return NewError(ErrInvalidInput, "The completed file path is not safe to delete.", err)
	}
	if err := os.Remove(filePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewError(ErrInvalidInput, "The completed file is already missing. Remove the record instead.", err)
		}
		return NewError(ErrInternal, "Could not delete the completed file.", err)
	}
	if err := s.repository.Delete(ctx, task.ID); err != nil {
		return repositoryError("remove", err)
	}
	s.clearDownloadSecrets(ctx, task.ID)
	return nil
}

func (s *DownloadService) completedDownload(ctx context.Context, id string) (download.Download, error) {
	id, err := validateID(id)
	if err != nil {
		return download.Download{}, NewError(ErrInvalidInput, "Invalid download identifier.", err)
	}
	task, err := s.repository.Get(ctx, id)
	if err != nil {
		return download.Download{}, repositoryError("remove", err)
	}
	if task.State != download.StateCompleted {
		return download.Download{}, NewError(ErrInvalidInput, "Only completed downloads can be removed.", nil)
	}
	return task, nil
}

func (s *DownloadService) clearDownloadSecrets(ctx context.Context, id string) {
	if s.profileResolver != nil {
		_ = s.profileResolver.ClearDownloadSecrets(ctx, id)
	}
}

func completedFilePath(task download.Download) (string, error) {
	filePath := filepath.Clean(task.DestinationPath)
	if !filepath.IsAbs(filePath) || filepath.Base(filePath) != task.FileName {
		return "", errors.New("completed file path does not match its download record")
	}
	directory, err := fluxfs.ValidateDestinationDirectory(filepath.Dir(filePath))
	if err != nil {
		return "", err
	}
	if filepath.Clean(filepath.Join(directory, task.FileName)) != filePath {
		return "", errors.New("completed file is outside its destination directory")
	}
	info, err := os.Lstat(filePath)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", errors.New("completed file path is a directory")
	}
	return filePath, nil
}

func (s *DownloadService) SetGlobalBandwidthLimit(limit int64) error {
	if limit < 0 {
		return NewError(ErrInvalidInput, "Bandwidth limit cannot be negative.", nil)
	}
	if err := s.engine.SetGlobalBandwidthLimit(limit); err != nil {
		return NewError(ErrInvalidInput, "Could not apply the global bandwidth limit.", err)
	}
	return nil
}

func (s *DownloadService) SetDownloadBandwidthLimit(ctx context.Context, id string, limit int64) error {
	id, err := validateID(id)
	if err != nil {
		return NewError(ErrInvalidInput, "Invalid download identifier.", err)
	}
	if limit < 0 {
		return NewError(ErrInvalidInput, "Bandwidth limit cannot be negative.", nil)
	}
	task, err := s.repository.Get(ctx, id)
	if err != nil {
		return repositoryError("configure", err)
	}
	task.BandwidthLimit = limit
	if err := s.repository.Save(ctx, task); err != nil {
		return repositoryError("configure", err)
	}
	s.publishUpdated(downloadToDTO(task))
	return nil
}

func (s *DownloadService) StartQueue(ctx context.Context, queueID string, retryFailed bool) error {
	queueID, err := validateID(queueID)
	if err != nil {
		return NewError(ErrInvalidInput, "Invalid queue identifier.", err)
	}
	tasks, err := s.repository.List(ctx)
	if err != nil {
		return repositoryError("start queue", err)
	}
	for _, task := range tasks {
		if task.QueueID != queueID {
			continue
		}
		switch task.State {
		case download.StateQueued:
			_ = s.Start(ctx, task.ID)
		case download.StatePaused:
			_ = s.Resume(ctx, task.ID)
		case download.StateFailed:
			if retryFailed {
				_ = s.Start(ctx, task.ID)
			}
		}
	}
	return nil
}

func (s *DownloadService) StopQueue(ctx context.Context, queueID string) error {
	queueID, err := validateID(queueID)
	if err != nil {
		return NewError(ErrInvalidInput, "Invalid queue identifier.", err)
	}
	tasks, err := s.repository.List(ctx)
	if err != nil {
		return repositoryError("stop queue", err)
	}
	for _, task := range tasks {
		if task.QueueID == queueID && task.State == download.StateDownloading {
			_ = s.Pause(ctx, task.ID)
		}
	}
	return nil
}

func (s *DownloadService) RetryFailed(ctx context.Context, queueID string) error {
	tasks, err := s.repository.List(ctx)
	if err != nil {
		return repositoryError("retry failed", err)
	}
	for _, task := range tasks {
		if task.State == download.StateFailed && (queueID == "" || task.QueueID == queueID) {
			_ = s.Start(ctx, task.ID)
		}
	}
	return nil
}

func (s *DownloadService) WaitForIdle(ctx context.Context, queueID string) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		tasks, err := s.repository.List(ctx)
		if err != nil {
			return repositoryError("wait for", err)
		}
		busy := false
		for _, task := range tasks {
			if queueID != "" && task.QueueID != queueID {
				continue
			}
			switch task.State {
			case download.StateQueued, download.StateProbing, download.StatePreparing, download.StateDownloading, download.StatePausing, download.StateRetrying:
				busy = true
			}
			if busy {
				break
			}
		}
		if !busy {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s *DownloadService) enqueue(job downloadJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.queued[job.id]; exists {
		return nil
	}
	if _, exists := s.running[job.id]; exists {
		return nil
	}
	if len(s.pending) >= maxQueuedDownloads {
		return NewError(ErrUnavailable, "The download queue is full.", nil)
	}
	task, err := s.repository.Get(s.ctx, job.id)
	if err != nil {
		return repositoryError("queue", err)
	}
	job.queueID = task.QueueID
	job.priority = task.Priority
	job.maxParallel = defaultDownloadWorkers
	if job.queueID == "" {
		job.queueID = "__default"
		job.maxParallel = 3
	}
	if s.organization != nil && task.QueueID != "" {
		queue, queueErr := s.organization.GetQueue(s.ctx, task.QueueID)
		if queueErr != nil {
			return NewError(ErrInvalidInput, "The assigned queue no longer exists.", queueErr)
		}
		if !queue.Enabled {
			return NewError(ErrUnavailable, "The assigned queue is stopped.", nil)
		}
		job.priority += queue.Priority
		job.maxParallel = queue.MaxParallel
		if queue.Sequential {
			job.maxParallel = 1
		}
	}
	s.queued[job.id] = struct{}{}
	s.pending = append(s.pending, job)
	s.signalWorkers()
	return nil
}

func (s *DownloadService) runningControl(id string) *runControl {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running[id]
}

func (s *DownloadService) worker() {
	defer s.wg.Done()
	for {
		job, ok := s.nextJob()
		if !ok {
			return
		}
		s.mu.Lock()
		runCtx, cancel := context.WithCancel(s.ctx)
		control := &runControl{cancel: cancel}
		s.running[job.id] = control
		s.mu.Unlock()

		s.process(runCtx, job, control)
		cancel()
		s.mu.Lock()
		delete(s.running, job.id)
		s.queueRunning[job.queueID]--
		s.mu.Unlock()
		s.signalWorkers()
	}
}

func (s *DownloadService) nextJob() (downloadJob, bool) {
	for {
		s.mu.Lock()
		best := -1
		for index, job := range s.pending {
			if _, queued := s.queued[job.id]; !queued {
				continue
			}
			if s.queueRunning[job.queueID] >= job.maxParallel {
				continue
			}
			if best == -1 || job.priority > s.pending[best].priority {
				best = index
			}
		}
		if best >= 0 {
			job := s.pending[best]
			s.pending = append(s.pending[:best], s.pending[best+1:]...)
			delete(s.queued, job.id)
			s.queueRunning[job.queueID]++
			s.mu.Unlock()
			s.signalWorkers()
			return job, true
		}
		s.mu.Unlock()
		select {
		case <-s.ctx.Done():
			return downloadJob{}, false
		case <-s.wake:
		}
	}
}

func (s *DownloadService) signalWorkers() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *DownloadService) process(ctx context.Context, job downloadJob, control *runControl) {
	task, err := s.repository.Get(ctx, job.id)
	if err != nil {
		return
	}
	if job.action == actionStart {
		if task.State != download.StateQueued || s.transitionAndSave(ctx, &task, download.StateProbing) != nil {
			return
		}
		if s.profileResolver != nil {
			_, options, resolveErr := s.profileResolver.Resolve(ctx, task.URL, task.SiteProfileID, task.ID)
			if resolveErr != nil {
				s.handlePreEngineError(context.WithoutCancel(ctx), &task, control, resolveErr)
				return
			}
			task.RequestOptions = options
		}
		probe, probeErr := s.prober.ProbeWithOptions(ctx, task.URL, task.RequestOptions)
		if probeErr != nil {
			s.handlePreEngineError(context.WithoutCancel(ctx), &task, control, probeErr)
			return
		}
		applyProbe(&task, probe)
		if planErr := planTaskSegments(&task); planErr != nil {
			s.finishWithError(context.WithoutCancel(ctx), &task, planErr)
			return
		}
		task.DownloadedBytes = 0
	} else {
		if task.State != download.StatePaused && task.State != download.StateRetrying {
			return
		}
		if s.profileResolver != nil {
			_, options, resolveErr := s.profileResolver.Resolve(ctx, task.URL, task.SiteProfileID, task.ID)
			if resolveErr != nil {
				s.handlePreEngineError(context.WithoutCancel(ctx), &task, control, resolveErr)
				return
			}
			task.RequestOptions = options
		}
		probe, probeErr := s.prober.ProbeWithOptions(ctx, task.URL, task.RequestOptions)
		if probeErr != nil {
			s.handlePreEngineError(context.WithoutCancel(ctx), &task, control, probeErr)
			return
		}
		if resumeErr := validateResume(task, probe); resumeErr != nil {
			if errors.Is(resumeErr, download.ErrRangeUnsupported) {
				task.RangeSupported = false
			}
			s.finishWithError(context.WithoutCancel(ctx), &task, resumeErr)
			return
		}
		applyProbe(&task, probe)
		if reconcileErr := reconcileTemporaryFile(&task); reconcileErr != nil {
			s.finishWithError(context.WithoutCancel(ctx), &task, reconcileErr)
			return
		}
	}
	if err := s.transitionAndSave(ctx, &task, download.StatePreparing); err != nil {
		return
	}
	now := time.Now().UTC()
	if task.StartedAt == nil {
		task.StartedAt = &now
	}
	for index := range task.Segments {
		if task.Segments[index].CurrentByte <= task.Segments[index].EndByte {
			task.Segments[index].State = download.SegmentDownloading
		}
	}
	if err := s.transitionAndSave(ctx, &task, download.StateDownloading); err != nil {
		return
	}
	if s.organization != nil && task.QueueID != "" {
		queue, queueErr := s.organization.GetQueue(ctx, task.QueueID)
		if queueErr != nil {
			s.finishWithError(context.WithoutCancel(ctx), &task, queueErr)
			return
		}
		task.QueueBandwidthLimit = queue.BandwidthLimit
	}

	lastCheckpoint := time.Now()
	err = s.engine.Download(ctx, task, func(progress download.Progress) {
		task.DownloadedBytes = progress.DownloadedBytes
		task.Segments = append([]download.Segment(nil), progress.Segments...)
		if len(progress.Segments) == 1 && task.Connections != 1 {
			task.Connections = 1
		}
		s.bus.Publish(events.Event{Type: events.DownloadProgress, Data: progress})
		if stopIntent(control.intent.Load()) == intentNone && time.Since(lastCheckpoint) >= checkpointInterval {
			if saveErr := s.repository.Save(context.WithoutCancel(ctx), task); saveErr == nil {
				lastCheckpoint = time.Now()
			}
		}
	})
	control.finished.Store(true)
	control.mu.Lock()
	intent := stopIntent(control.intent.Load())
	control.mu.Unlock()
	if err != nil {
		switch {
		case intent == intentPause:
			_ = task.Transition(download.StatePausing)
			s.finishPause(context.WithoutCancel(ctx), task)
		case intent == intentCancel:
			s.finishCancel(context.WithoutCancel(ctx), task.ID)
		case s.ctx.Err() != nil:
			s.checkpointInterruptedTask(context.WithoutCancel(ctx), task)
		default:
			if errors.Is(err, download.ErrRangeUnsupported) {
				task.RangeSupported = false
			}
			s.finishWithError(context.WithoutCancel(ctx), &task, err)
		}
		return
	}
	completedAt := time.Now().UTC()
	task.CompletedAt = &completedAt
	if task.TotalBytes >= 0 {
		task.DownloadedBytes = task.TotalBytes
	}
	for index := range task.Segments {
		task.Segments[index].CurrentByte = task.Segments[index].EndByte + 1
		task.Segments[index].State = download.SegmentCompleted
		task.Segments[index].LastError = ""
	}
	task.LastError = ""
	_ = s.transitionAndSave(context.WithoutCancel(ctx), &task, download.StateCompleted)
	if s.profileResolver != nil {
		_ = s.profileResolver.ClearDownloadSecrets(context.WithoutCancel(ctx), task.ID)
	}
}

func (s *DownloadService) handlePreEngineError(ctx context.Context, task *download.Download, control *runControl, cause error) {
	control.finished.Store(true)
	control.mu.Lock()
	intent := stopIntent(control.intent.Load())
	control.mu.Unlock()
	switch {
	case intent == intentCancel:
		s.finishCancel(ctx, task.ID)
	case s.ctx.Err() != nil:
		s.checkpointInterrupted(ctx, task.ID)
	default:
		s.finishWithError(ctx, task, cause)
	}
}

func (s *DownloadService) finishPause(ctx context.Context, task download.Download) {
	if err := reconcileTemporaryFile(&task); err != nil {
		s.finishWithError(ctx, &task, err)
		return
	}
	if task.State != download.StatePausing || task.Transition(download.StatePaused) != nil {
		return
	}
	for index := range task.Segments {
		if task.Segments[index].State != download.SegmentCompleted {
			task.Segments[index].State = download.SegmentPaused
		}
	}
	task.LastError = ""
	if err := s.repository.Save(ctx, task); err == nil {
		s.publishUpdated(downloadToDTO(task))
	}
}

func (s *DownloadService) finishCancel(ctx context.Context, id string) {
	task, err := s.repository.Get(ctx, id)
	if err != nil {
		return
	}
	if err := task.Transition(download.StateCancelled); err != nil {
		return
	}
	_ = os.Remove(task.TempPath)
	task.DownloadedBytes = 0
	for index := range task.Segments {
		task.Segments[index].CurrentByte = task.Segments[index].StartByte
		task.Segments[index].State = download.SegmentPending
	}
	task.LastError = "Cancelled by user."
	if err := s.repository.Save(ctx, task); err == nil {
		s.publishUpdated(downloadToDTO(task))
	}
	if s.profileResolver != nil {
		_ = s.profileResolver.ClearDownloadSecrets(ctx, id)
	}
}

func (s *DownloadService) checkpointInterrupted(ctx context.Context, id string) {
	task, err := s.repository.Get(ctx, id)
	if err != nil {
		return
	}
	_ = reconcileTemporaryFile(&task)
	_ = s.repository.Save(ctx, task)
}

func (s *DownloadService) checkpointInterruptedTask(ctx context.Context, task download.Download) {
	_ = reconcileTemporaryFile(&task)
	_ = s.repository.Save(ctx, task)
}

func (s *DownloadService) recoverTask(ctx context.Context, task *download.Download) error {
	if err := reconcileTemporaryFile(task); err != nil {
		if recoverErr := task.Recover(download.StateFailed); recoverErr != nil {
			return recoverErr
		}
		task.LastError = "Temporary file recovery failed. Restart the download."
		markSegments(task.Segments, download.SegmentFailed)
	} else if task.DownloadedBytes == 0 {
		if err := task.Recover(download.StateQueued); err != nil {
			return err
		}
		markSegments(task.Segments, download.SegmentPending)
	} else if task.RangeSupported {
		if err := task.Recover(download.StatePaused); err != nil {
			return err
		}
		markIncompleteSegments(task.Segments, download.SegmentPaused)
		task.LastError = "Recovered after an interrupted application session."
	} else {
		if err := task.Recover(download.StateFailed); err != nil {
			return err
		}
		markIncompleteSegments(task.Segments, download.SegmentFailed)
		task.LastError = "This server cannot resume the partial file. Restart the download."
	}
	if err := s.repository.Save(ctx, *task); err != nil {
		return err
	}
	s.publishUpdated(downloadToDTO(*task))
	return nil
}

func (s *DownloadService) transitionAndSave(ctx context.Context, task *download.Download, state download.State) error {
	if err := task.Transition(state); err != nil {
		return err
	}
	if err := s.repository.Save(ctx, *task); err != nil {
		return err
	}
	s.publishUpdated(downloadToDTO(*task))
	return nil
}

func (s *DownloadService) finishWithError(ctx context.Context, task *download.Download, cause error) {
	if err := task.Transition(download.StateFailed); err != nil {
		return
	}
	markIncompleteSegments(task.Segments, download.SegmentFailed)
	task.LastError = downloadErrorMessage(cause)
	for index := range task.Segments {
		if task.Segments[index].State != download.SegmentCompleted {
			task.Segments[index].LastError = task.LastError
		}
	}
	if err := s.repository.Save(ctx, *task); err == nil {
		s.publishUpdated(downloadToDTO(*task))
	}
}

func (s *DownloadService) publishUpdated(dto DownloadDTO) {
	s.bus.Publish(events.Event{Type: events.DownloadUpdated, Data: dto})
}

func applyProbe(task *download.Download, probe download.ProbeResult) {
	task.FinalURL = probe.FinalURL
	task.TotalBytes = probe.TotalBytes
	task.RangeSupported = probe.RangeSupported
	task.ETag = probe.ETag
	task.LastModified = probe.LastModified
	task.MIMEType = probe.MIMEType
}

func planTaskSegments(task *download.Download) error {
	connections := 1
	if task.RangeSupported && task.TotalBytes > 0 {
		connections = task.Connections
	}
	if task.TotalBytes > 0 {
		segments, err := download.PlanSegments(task.ID, task.TempPath, task.TotalBytes, connections)
		if err != nil {
			return err
		}
		task.Segments = segments
		if !task.RangeSupported {
			task.Connections = 1
		}
		return nil
	}
	task.Connections = 1
	task.Segments = []download.Segment{{
		ID: task.ID + ":0", DownloadID: task.ID, Index: 0, StartByte: 0, EndByte: -1,
		CurrentByte: 0, State: download.SegmentPending, TempPath: task.TempPath,
	}}
	return nil
}

func markSegments(segments []download.Segment, state download.SegmentState) {
	for index := range segments {
		segments[index].State = state
	}
}

func markIncompleteSegments(segments []download.Segment, state download.SegmentState) {
	for index := range segments {
		if segments[index].State != download.SegmentCompleted {
			segments[index].State = state
		}
	}
}

func validateResume(task download.Download, probe download.ProbeResult) error {
	if task.DownloadedBytes > 0 && !probe.RangeSupported {
		return download.ErrRangeUnsupported
	}
	if task.TotalBytes >= 0 && probe.TotalBytes >= 0 && task.TotalBytes != probe.TotalBytes {
		return ErrRemoteChanged
	}
	if task.ETag != "" && probe.ETag != "" && task.ETag != probe.ETag {
		return ErrRemoteChanged
	}
	if task.LastModified != "" && probe.LastModified != "" && task.LastModified != probe.LastModified {
		return ErrRemoteChanged
	}
	return nil
}

func reconcileTemporaryFile(task *download.Download) error {
	info, err := os.Stat(task.TempPath)
	if err != nil {
		if os.IsNotExist(err) && task.CompletedSegmentBytes() == 0 {
			return nil
		}
		return fmt.Errorf("inspect partial file: %w", err)
	}
	size := info.Size()
	if task.TotalBytes >= 0 {
		if len(task.Segments) > 1 && size != task.TotalBytes {
			return fmt.Errorf("preallocated partial file size does not match the expected download")
		}
		if size > task.TotalBytes {
			return fmt.Errorf("partial file is larger than the expected download")
		}
		if len(task.Segments) == 1 && size != task.TotalBytes {
			task.Segments[0].CurrentByte = task.Segments[0].StartByte + size
		}
	} else if len(task.Segments) == 1 {
		task.Segments[0].CurrentByte = task.Segments[0].StartByte + size
	}
	if task.TotalBytes >= 0 {
		if err := download.ValidateSegments(task.Segments, task.TotalBytes); err != nil {
			return err
		}
	}
	task.DownloadedBytes = task.CompletedSegmentBytes()
	return nil
}

func isRecoverableState(state download.State) bool {
	switch state {
	case download.StateProbing, download.StatePreparing, download.StateDownloading, download.StatePausing, download.StateRetrying:
		return true
	default:
		return false
	}
}

func validateID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 64 {
		return "", fmt.Errorf("invalid identifier")
	}
	return id, nil
}

func newID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16])
}

func fileNameFromURL(parsed *url.URL) string {
	name := path.Base(parsed.Path)
	if name == "" || name == "." || name == "/" {
		name = "download"
	}
	if decoded, err := url.PathUnescape(name); err == nil {
		name = decoded
	}
	return fluxfs.SanitizeFileName(name)
}

func downloadToDTO(task download.Download) DownloadDTO {
	restartRequired := (task.State == download.StateFailed && strings.Contains(strings.ToLower(task.LastError), "restart")) ||
		(task.State == download.StatePaused && task.DownloadedBytes > 0 && !task.RangeSupported)
	return DownloadDTO{
		ID: task.ID, URL: task.URL, FinalURL: task.FinalURL, FileName: task.FileName,
		DestinationPath: task.DestinationPath, TempPath: task.TempPath, State: string(task.State),
		TotalBytes: task.TotalBytes, DownloadedBytes: task.DownloadedBytes,
		RangeSupported: task.RangeSupported, RestartRequired: restartRequired, MIMEType: task.MIMEType,
		CreatedAt: formatDTOTime(task.CreatedAt), StartedAt: formatDTOOptionalTime(task.StartedAt),
		CompletedAt: formatDTOOptionalTime(task.CompletedAt), LastError: task.LastError,
		RetryCount: task.RetryCount, Connections: task.Connections, SegmentCount: len(task.Segments), BandwidthLimit: task.BandwidthLimit,
		CategoryID: task.CategoryID, QueueID: task.QueueID, QueuePosition: task.QueuePosition, Priority: task.Priority,
		SiteProfileID: task.SiteProfileID,
	}
}

func probeToDTO(result download.ProbeResult) ProbeDTO {
	return ProbeDTO{
		URL: result.URL, FinalURL: result.FinalURL, FileName: result.FileName,
		TotalBytes: result.TotalBytes, MIMEType: result.MIMEType, ETag: result.ETag,
		LastModified: result.LastModified, RangeSupported: result.RangeSupported,
		ExecutableWarning: fluxfs.IsExecutableLike(result.FileName),
	}
}

func formatDTOTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func formatDTOOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := formatDTOTime(*value)
	return &formatted
}

func repositoryError(action string, err error) *Error {
	if errors.Is(err, download.ErrNotFound) {
		return NewError(ErrInvalidInput, "Download not found.", err)
	}
	return NewError(ErrInternal, fmt.Sprintf("Could not %s the download.", action), err)
}

func probeErrorMessage(err error) string {
	var statusError *download.HTTPStatusError
	if errors.As(err, &statusError) {
		return fmt.Sprintf("The server returned HTTP %d.", statusError.StatusCode)
	}
	return "FluxDM could not inspect that URL."
}

func downloadErrorMessage(err error) string {
	var statusError *download.HTTPStatusError
	if errors.As(err, &statusError) {
		return fmt.Sprintf("The server returned HTTP %d.", statusError.StatusCode)
	}
	if errors.Is(err, ErrRemoteChanged) {
		return "The remote file changed. Restart the download to continue safely."
	}
	if errors.Is(err, download.ErrRangeUnsupported) {
		return "The server no longer supports resuming. Restart the download."
	}
	if errors.Is(err, download.ErrRangeUnreliable) {
		return "The server returned inconsistent byte ranges. Restart the download."
	}
	if errors.Is(err, download.ErrCancelled) || errors.Is(err, context.Canceled) {
		return "Cancelled by user."
	}
	return "The download failed. Check the connection and destination, then retry."
}
