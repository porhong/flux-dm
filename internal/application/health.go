package application

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const Version = "1.0.0"

type Paths struct {
	DataDir string
}

func DefaultPaths() (Paths, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, err
	}
	return Paths{DataDir: filepath.Join(root, "FluxDM")}, nil
}

// DefaultDownloadDirectory returns the user's standard Downloads directory,
// creating it when it has not yet been created by Windows or another app.
func DefaultDownloadDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return defaultDownloadDirectoryForHome(home)
}

func defaultDownloadDirectoryForHome(home string) (string, error) {
	if strings.TrimSpace(home) == "" {
		return "", errors.New("user home directory is required")
	}
	directory := filepath.Join(home, "Downloads")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", err
	}
	return directory, nil
}

type HealthStatus struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Platform  string `json:"platform"`
	CheckedAt string `json:"checkedAt"`
}

type ReadyEvent struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Message string `json:"message"`
}

func NewHealthStatus() HealthStatus {
	return HealthStatus{
		Status:    "ok",
		Version:   Version,
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
		CheckedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
}
