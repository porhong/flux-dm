package application

import (
	"os"
	"path/filepath"
	"runtime"
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
