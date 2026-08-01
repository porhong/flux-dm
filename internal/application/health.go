package application

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const Version = "1.0.0"

// ReleaseVersion is overridden by release builds and retains the RC suffix
// that Windows file-version metadata cannot represent.
var ReleaseVersion = Version

// PortableMode is set to "true" by portable release builds. It keeps the
// executable, its data, and its browser-integration manifest together while
// development and installed builds retain the standard Windows config path.
var PortableMode = "false"

func IsPortableBuild() bool { return strings.EqualFold(PortableMode, "true") }

// Update public keys are injected at build time. Keeping only public keys in
// the binary permits offline verification without shipping signing secrets.
var StableUpdatePublicKeyBase64 string
var PreviewUpdatePublicKeyBase64 string

func UpdatePublicKeys() (ed25519.PublicKey, ed25519.PublicKey, error) {
	stable, err := base64.StdEncoding.DecodeString(StableUpdatePublicKeyBase64)
	if err != nil || len(stable) != ed25519.PublicKeySize {
		return nil, nil, errors.New("stable update public key is not configured")
	}
	preview, err := base64.StdEncoding.DecodeString(PreviewUpdatePublicKeyBase64)
	if err != nil || len(preview) != ed25519.PublicKeySize {
		return nil, nil, errors.New("preview update public key is not configured")
	}
	return ed25519.PublicKey(stable), ed25519.PublicKey(preview), nil
}

type Paths struct {
	DataDir string
}

func DefaultPaths() (Paths, error) {
	if IsPortableBuild() {
		executable, err := os.Executable()
		if err != nil {
			return Paths{}, err
		}
		return portablePaths(executable)
	}
	root, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, err
	}
	return Paths{DataDir: filepath.Join(root, "FluxDM")}, nil
}

func portablePaths(executable string) (Paths, error) {
	if strings.TrimSpace(executable) == "" || !filepath.IsAbs(executable) {
		return Paths{}, errors.New("portable executable path is required")
	}
	return Paths{DataDir: filepath.Join(filepath.Dir(executable), "data")}, nil
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
		Version:   ReleaseVersion,
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
		CheckedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
}
