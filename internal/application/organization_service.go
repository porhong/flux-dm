package application

import (
	"context"
	"strings"
	"time"

	"github.com/fluxdm/fluxdm/internal/download"
	fluxfs "github.com/fluxdm/fluxdm/internal/filesystem"
	"github.com/fluxdm/fluxdm/internal/organization"
)

type SaveCategoryInput struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Extensions     []string `json:"extensions"`
	DestinationDir string   `json:"destinationDir"`
	Priority       int      `json:"priority"`
}

type SaveQueueInput struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Priority       int    `json:"priority"`
	MaxParallel    int    `json:"maxParallel"`
	MaxConnections int    `json:"maxConnections"`
	BandwidthLimit int64  `json:"bandwidthLimit"`
	Sequential     bool   `json:"sequential"`
	Enabled        bool   `json:"enabled"`
}

type AssignDownloadsInput struct {
	DownloadIDs []string `json:"downloadIds"`
	CategoryID  string   `json:"categoryId"`
	QueueID     string   `json:"queueId"`
	Priority    int      `json:"priority"`
}

type OrganizationService struct {
	repository organization.Repository
	downloads  download.Repository
}

func NewOrganizationService(repository organization.Repository, downloads download.Repository) *OrganizationService {
	return &OrganizationService{repository: repository, downloads: downloads}
}

func (s *OrganizationService) ListCategories(ctx context.Context) ([]organization.Category, error) {
	items, err := s.repository.ListCategories(ctx)
	if err != nil {
		return nil, NewError(ErrInternal, "Could not list categories.", err)
	}
	return items, nil
}

func (s *OrganizationService) SaveCategory(ctx context.Context, input SaveCategoryInput) (organization.Category, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 80 {
		return organization.Category{}, NewError(ErrInvalidInput, "Category name must contain 1 to 80 characters.", nil)
	}
	extensions := organization.NormalizeExtensions(input.Extensions)
	if len(extensions) == 0 || len(extensions) > 100 {
		return organization.Category{}, NewError(ErrInvalidInput, "Add between 1 and 100 valid extensions.", nil)
	}
	destination := strings.TrimSpace(input.DestinationDir)
	if destination != "" {
		validated, err := fluxfs.ValidateDestinationDirectory(destination)
		if err != nil {
			return organization.Category{}, NewError(ErrInvalidInput, "Choose an existing category destination.", err)
		}
		destination = validated
	}
	if input.Priority < -1000 || input.Priority > 1000 {
		return organization.Category{}, NewError(ErrInvalidInput, "Priority must be between -1000 and 1000.", nil)
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = newID()
	} else if _, err := validateID(id); err != nil {
		return organization.Category{}, NewError(ErrInvalidInput, "Invalid category identifier.", err)
	}
	item := organization.Category{ID: id, Name: name, Extensions: extensions, DestinationDir: destination, Priority: input.Priority, CreatedAt: time.Now().UTC()}
	if err := s.repository.SaveCategory(ctx, item); err != nil {
		return organization.Category{}, NewError(ErrInternal, "Could not save category.", err)
	}
	return item, nil
}

func (s *OrganizationService) DeleteCategory(ctx context.Context, id string) error {
	id, err := validateID(id)
	if err != nil {
		return NewError(ErrInvalidInput, "Invalid category identifier.", err)
	}
	if err := s.repository.DeleteCategory(ctx, id); err != nil {
		return repositoryError("delete category", err)
	}
	return nil
}

func (s *OrganizationService) ListQueues(ctx context.Context) ([]organization.Queue, error) {
	items, err := s.repository.ListQueues(ctx)
	if err != nil {
		return nil, NewError(ErrInternal, "Could not list queues.", err)
	}
	return items, nil
}

func (s *OrganizationService) SaveQueue(ctx context.Context, input SaveQueueInput) (organization.Queue, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 80 {
		return organization.Queue{}, NewError(ErrInvalidInput, "Queue name must contain 1 to 80 characters.", nil)
	}
	if input.Priority < -1000 || input.Priority > 1000 {
		return organization.Queue{}, NewError(ErrInvalidInput, "Priority must be between -1000 and 1000.", nil)
	}
	if input.MaxParallel < 1 || input.MaxParallel > 16 {
		return organization.Queue{}, NewError(ErrInvalidInput, "Max parallel downloads must be between 1 and 16.", nil)
	}
	if !download.ValidConnectionCount(input.MaxConnections) {
		return organization.Queue{}, NewError(ErrInvalidInput, "Choose 1, 2, 4, 8, or 16 connections.", nil)
	}
	if input.BandwidthLimit < 0 {
		return organization.Queue{}, NewError(ErrInvalidInput, "Bandwidth limit cannot be negative.", nil)
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = newID()
	} else if _, err := validateID(id); err != nil {
		return organization.Queue{}, NewError(ErrInvalidInput, "Invalid queue identifier.", err)
	}
	item := organization.Queue{ID: id, Name: name, Priority: input.Priority, MaxParallel: input.MaxParallel, MaxConnections: input.MaxConnections, BandwidthLimit: input.BandwidthLimit, Sequential: input.Sequential, Enabled: input.Enabled, CreatedAt: time.Now().UTC()}
	if err := s.repository.SaveQueue(ctx, item); err != nil {
		return organization.Queue{}, NewError(ErrInternal, "Could not save queue.", err)
	}
	return item, nil
}

func (s *OrganizationService) DeleteQueue(ctx context.Context, id string) error {
	id, err := validateID(id)
	if err != nil {
		return NewError(ErrInvalidInput, "Invalid queue identifier.", err)
	}
	if err := s.repository.DeleteQueue(ctx, id); err != nil {
		return repositoryError("delete queue", err)
	}
	return nil
}

func (s *OrganizationService) SetQueueEnabled(ctx context.Context, id string, enabled bool) error {
	id, err := validateID(id)
	if err != nil {
		return NewError(ErrInvalidInput, "Invalid queue identifier.", err)
	}
	item, err := s.repository.GetQueue(ctx, id)
	if err != nil {
		return repositoryError("configure queue", err)
	}
	item.Enabled = enabled
	if err := s.repository.SaveQueue(ctx, item); err != nil {
		return NewError(ErrInternal, "Could not configure queue.", err)
	}
	return nil
}

func (s *OrganizationService) AssignDownloads(ctx context.Context, input AssignDownloadsInput) error {
	if len(input.DownloadIDs) == 0 || len(input.DownloadIDs) > 1000 {
		return NewError(ErrInvalidInput, "Choose between 1 and 1000 downloads.", nil)
	}
	if input.Priority < -1000 || input.Priority > 1000 {
		return NewError(ErrInvalidInput, "Priority must be between -1000 and 1000.", nil)
	}
	if input.CategoryID != "" {
		if _, err := validateID(input.CategoryID); err != nil {
			return NewError(ErrInvalidInput, "Invalid category identifier.", err)
		}
	}
	if input.QueueID != "" {
		if _, err := s.repository.GetQueue(ctx, input.QueueID); err != nil {
			return NewError(ErrInvalidInput, "Choose an existing queue.", err)
		}
	}
	for index, id := range input.DownloadIDs {
		id, err := validateID(id)
		if err != nil {
			return NewError(ErrInvalidInput, "Invalid download identifier.", err)
		}
		task, err := s.downloads.Get(ctx, id)
		if err != nil {
			return repositoryError("organize", err)
		}
		task.CategoryID = input.CategoryID
		task.QueueID = input.QueueID
		task.Priority = input.Priority
		task.QueuePosition = time.Now().UTC().UnixNano() + int64(index)
		if err := s.downloads.Save(ctx, task); err != nil {
			return repositoryError("organize", err)
		}
	}
	return nil
}
