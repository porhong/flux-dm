package application

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fluxdm/fluxdm/internal/download"
	"github.com/fluxdm/fluxdm/internal/events"
	fluxfs "github.com/fluxdm/fluxdm/internal/filesystem"
)

const maxFileManagementBatch = 1000

type CompletedFileOperations interface {
	Open(string) error
	Reveal(string) error
	Recycle(string) error
	Rename(string, string) (string, error)
	Move(string, string) (string, error)
}

type MoveCompletedDownloadsInput struct {
	DownloadIDs    []string `json:"downloadIds"`
	DestinationDir string   `json:"destinationDir"`
}

type CompletedFileOperationResult struct {
	Updated    []DownloadDTO                   `json:"updated"`
	RemovedIDs []string                        `json:"removedIds"`
	SkippedIDs []string                        `json:"skippedIds"`
	Failures   []CompletedFileOperationFailure `json:"failures"`
}

type CompletedFileOperationFailure struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

type FileManagementService struct {
	repository download.Repository
	files      CompletedFileOperations
	bus        *events.Bus
}

func NewFileManagementService(repository download.Repository, files CompletedFileOperations, bus *events.Bus) *FileManagementService {
	return &FileManagementService{repository: repository, files: files, bus: bus}
}

func (s *FileManagementService) Open(ctx context.Context, id string) error {
	task, err := s.completedDownload(ctx, id)
	if err != nil {
		return err
	}
	if err := s.files.Open(task.DestinationPath); err != nil {
		return fileOperationError("open", err)
	}
	return nil
}

func (s *FileManagementService) Reveal(ctx context.Context, id string) error {
	task, err := s.completedDownload(ctx, id)
	if err != nil {
		return err
	}
	if err := s.files.Reveal(task.DestinationPath); err != nil {
		return fileOperationError("reveal", err)
	}
	return nil
}

func (s *FileManagementService) Rename(ctx context.Context, id, requestedName string) (DownloadDTO, error) {
	task, err := s.completedDownload(ctx, id)
	if err != nil {
		return DownloadDTO{}, err
	}
	name := strings.TrimSpace(requestedName)
	if name == "" || name != fluxfs.SanitizeFileName(name) {
		return DownloadDTO{}, NewError(ErrInvalidInput, "Enter a valid file name.", nil)
	}
	oldPath := task.DestinationPath
	newPath, err := s.files.Rename(oldPath, name)
	if err != nil {
		return DownloadDTO{}, fileOperationError("rename", err)
	}
	if err := s.persistPath(ctx, &task, newPath); err != nil {
		_, _ = s.files.Rename(newPath, filepath.Base(oldPath))
		return DownloadDTO{}, err
	}
	dto := downloadToDTO(task)
	s.publishUpdated(dto)
	return dto, nil
}

func (s *FileManagementService) Move(ctx context.Context, input MoveCompletedDownloadsInput) (CompletedFileOperationResult, error) {
	ids, directory, err := validateMoveInput(input)
	if err != nil {
		return CompletedFileOperationResult{}, err
	}
	result := newCompletedFileOperationResult()
	for _, id := range ids {
		task, getErr := s.repository.Get(ctx, id)
		if getErr != nil {
			result.Failures = append(result.Failures, failure(id, repositoryError("move", getErr)))
			continue
		}
		if task.State != download.StateCompleted {
			result.SkippedIDs = append(result.SkippedIDs, id)
			continue
		}
		oldPath := task.DestinationPath
		newPath, moveErr := s.files.Move(oldPath, directory)
		if moveErr != nil {
			result.Failures = append(result.Failures, failure(id, fileOperationError("move", moveErr)))
			continue
		}
		if persistErr := s.persistPath(ctx, &task, newPath); persistErr != nil {
			_, _ = s.files.Move(newPath, filepath.Dir(oldPath))
			result.Failures = append(result.Failures, failure(id, persistErr))
			continue
		}
		dto := downloadToDTO(task)
		s.publishUpdated(dto)
		result.Updated = append(result.Updated, dto)
	}
	return result, nil
}

func (s *FileManagementService) RemoveHistory(ctx context.Context, ids []string) (CompletedFileOperationResult, error) {
	ids, err := validateCompletedFileIDs(ids)
	if err != nil {
		return CompletedFileOperationResult{}, err
	}
	result := newCompletedFileOperationResult()
	for _, id := range ids {
		task, getErr := s.repository.Get(ctx, id)
		if getErr != nil {
			result.Failures = append(result.Failures, failure(id, repositoryError("remove history", getErr)))
			continue
		}
		if task.State != download.StateCompleted {
			result.SkippedIDs = append(result.SkippedIDs, id)
			continue
		}
		if deleteErr := s.repository.Delete(ctx, id); deleteErr != nil {
			result.Failures = append(result.Failures, failure(id, repositoryError("remove history", deleteErr)))
			continue
		}
		result.RemovedIDs = append(result.RemovedIDs, id)
	}
	return result, nil
}

func (s *FileManagementService) RecycleAndRemoveHistory(ctx context.Context, ids []string) (CompletedFileOperationResult, error) {
	ids, err := validateCompletedFileIDs(ids)
	if err != nil {
		return CompletedFileOperationResult{}, err
	}
	result := newCompletedFileOperationResult()
	for _, id := range ids {
		task, getErr := s.repository.Get(ctx, id)
		if getErr != nil {
			result.Failures = append(result.Failures, failure(id, repositoryError("delete", getErr)))
			continue
		}
		if task.State != download.StateCompleted {
			result.SkippedIDs = append(result.SkippedIDs, id)
			continue
		}
		if recycleErr := s.files.Recycle(task.DestinationPath); recycleErr != nil {
			result.Failures = append(result.Failures, failure(id, fileOperationError("delete", recycleErr)))
			continue
		}
		if deleteErr := s.repository.Delete(ctx, id); deleteErr != nil {
			result.Failures = append(result.Failures, failure(id, repositoryError("remove history", deleteErr)))
			continue
		}
		result.RemovedIDs = append(result.RemovedIDs, id)
	}
	return result, nil
}

func (s *FileManagementService) completedDownload(ctx context.Context, id string) (download.Download, error) {
	id, err := validateID(id)
	if err != nil {
		return download.Download{}, NewError(ErrInvalidInput, "Invalid download identifier.", err)
	}
	task, err := s.repository.Get(ctx, id)
	if err != nil {
		return download.Download{}, repositoryError("manage file", err)
	}
	if task.State != download.StateCompleted {
		return download.Download{}, NewError(ErrInvalidInput, "Only completed downloads can be managed as files.", nil)
	}
	return task, nil
}

func (s *FileManagementService) persistPath(ctx context.Context, task *download.Download, newPath string) error {
	task.DestinationPath = newPath
	task.TempPath = newPath + ".fluxpart"
	task.FileName = filepath.Base(newPath)
	if err := s.repository.Save(ctx, *task); err != nil {
		return repositoryError("update file path", err)
	}
	return nil
}

func validateMoveInput(input MoveCompletedDownloadsInput) ([]string, string, error) {
	ids, err := validateCompletedFileIDs(input.DownloadIDs)
	if err != nil {
		return nil, "", err
	}
	directory, err := fluxfs.ValidateDestinationDirectory(strings.TrimSpace(input.DestinationDir))
	if err != nil {
		return nil, "", NewError(ErrInvalidInput, "Choose an existing destination folder.", err)
	}
	return ids, directory, nil
}

func validateCompletedFileIDs(ids []string) ([]string, error) {
	if len(ids) == 0 || len(ids) > maxFileManagementBatch {
		return nil, NewError(ErrInvalidInput, "Choose between 1 and 1000 downloads.", nil)
	}
	result := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		validated, err := validateID(id)
		if err != nil {
			return nil, NewError(ErrInvalidInput, "Invalid download identifier.", err)
		}
		if _, exists := seen[validated]; !exists {
			seen[validated] = struct{}{}
			result = append(result, validated)
		}
	}
	return result, nil
}

func fileOperationError(action string, err error) error {
	return NewError(ErrInvalidInput, fmt.Sprintf("Could not %s the completed file.", action), err)
}

func failure(id string, err error) CompletedFileOperationFailure {
	return CompletedFileOperationFailure{ID: id, Message: err.Error()}
}

func newCompletedFileOperationResult() CompletedFileOperationResult {
	return CompletedFileOperationResult{Updated: make([]DownloadDTO, 0), RemovedIDs: make([]string, 0), SkippedIDs: make([]string, 0), Failures: make([]CompletedFileOperationFailure, 0)}
}

func (s *FileManagementService) publishUpdated(dto DownloadDTO) {
	if s.bus != nil {
		s.bus.Publish(events.Event{Type: events.DownloadUpdated, Data: dto})
	}
}
