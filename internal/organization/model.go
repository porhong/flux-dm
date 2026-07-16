package organization

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Category struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Extensions     []string  `json:"extensions"`
	DestinationDir string    `json:"destinationDir"`
	Priority       int       `json:"priority"`
	CreatedAt      time.Time `json:"createdAt"`
}

type Queue struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Priority       int       `json:"priority"`
	MaxParallel    int       `json:"maxParallel"`
	MaxConnections int       `json:"maxConnections"`
	BandwidthLimit int64     `json:"bandwidthLimit"`
	Sequential     bool      `json:"sequential"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"createdAt"`
}

type Repository interface {
	ListCategories(context.Context) ([]Category, error)
	SaveCategory(context.Context, Category) error
	DeleteCategory(context.Context, string) error
	ListQueues(context.Context) ([]Queue, error)
	GetQueue(context.Context, string) (Queue, error)
	SaveQueue(context.Context, Queue) error
	DeleteQueue(context.Context, string) error
}

// NormalizeExtensions makes matching stable across UI input, case, dots, and duplicates.
func NormalizeExtensions(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), ".")
		if value == "" || strings.ContainsAny(value, `/\\:*?"<>|`) {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

// MatchCategory picks the highest-priority rule; equal priorities are resolved by ID.
func MatchCategory(fileName string, categories []Category) *Category {
	extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(fileName)), ".")
	if extension == "" {
		return nil
	}
	matches := make([]Category, 0)
	for _, category := range categories {
		for _, candidate := range NormalizeExtensions(category.Extensions) {
			if candidate == extension {
				matches = append(matches, category)
				break
			}
		}
	}
	if len(matches) == 0 {
		return nil
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Priority == matches[j].Priority {
			return matches[i].ID < matches[j].ID
		}
		return matches[i].Priority > matches[j].Priority
	})
	return &matches[0]
}
