package browserintegration

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const nativeHostExecutableName = "FluxDM.NativeHost.exe"

var releaseVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-rc\.[0-9]+)?$`)

// InstallationStatus is deliberately limited to paths and safe repair state.
// It never exposes the browser bridge token or browser credentials.
type InstallationStatus struct {
	Ready         bool
	ExtensionPath string
	Message       string
}

type nativeHostConfig struct {
	Version     int    `json:"version"`
	DesktopPath string `json:"desktopPath"`
	DataDir     string `json:"dataDir"`
}

type nativeHostManifest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Path           string   `json:"path"`
	Type           string   `json:"type"`
	AllowedOrigins []string `json:"allowed_origins"`
}

// InstallPortable installs the companion files from an extracted portable
// bundle into LocalAppData and registers the fixed native host for Chromium
// browsers. Browser extension installation itself remains an explicit browser
// action; Chromium intentionally does not permit an application to bypass it.
func InstallPortable(desktopPath, releaseVersion string) (InstallationStatus, error) {
	if !filepath.IsAbs(desktopPath) || strings.TrimSpace(desktopPath) == "" {
		return InstallationStatus{}, errors.New("portable desktop path must be absolute")
	}
	if !releaseVersionPattern.MatchString(releaseVersion) {
		return InstallationStatus{}, errors.New("portable release version is invalid")
	}
	if info, err := os.Stat(desktopPath); err != nil || info.IsDir() {
		return InstallationStatus{}, errors.New("portable desktop executable is unavailable")
	}

	bundleRoot := filepath.Dir(desktopPath)
	sourceHost := filepath.Join(bundleRoot, nativeHostExecutableName)
	sourceExtension := filepath.Join(bundleRoot, "browser-extension")
	if info, err := os.Stat(sourceHost); err != nil || info.IsDir() {
		return InstallationStatus{Message: "The portable bundle is incomplete. Re-extract the complete FluxDM portable ZIP."}, errors.New("portable native host is unavailable")
	}
	if info, err := os.Stat(filepath.Join(sourceExtension, "manifest.json")); err != nil || info.IsDir() {
		return InstallationStatus{Message: "The portable bundle is incomplete. Re-extract the complete FluxDM portable ZIP."}, errors.New("portable browser extension is unavailable")
	}

	localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if !filepath.IsAbs(localAppData) {
		return InstallationStatus{}, errors.New("LocalAppData is unavailable")
	}
	targetRoot := filepath.Join(localAppData, "FluxDM", "browser-integration", releaseVersion)
	if err := os.MkdirAll(targetRoot, 0o700); err != nil {
		return InstallationStatus{}, fmt.Errorf("create browser integration directory: %w", err)
	}
	targetHost := filepath.Join(targetRoot, nativeHostExecutableName)
	targetExtension := filepath.Join(targetRoot, "browser-extension")
	if err := copyFile(sourceHost, targetHost); err != nil {
		return InstallationStatus{}, fmt.Errorf("copy native host: %w", err)
	}
	if err := copyExtension(sourceExtension, targetExtension); err != nil {
		return InstallationStatus{}, fmt.Errorf("copy browser extension: %w", err)
	}
	if err := writeFileJSON(filepath.Join(targetRoot, "fluxdm-browser-host.json"), nativeHostConfig{
		Version: 1, DesktopPath: desktopPath, DataDir: filepath.Join(bundleRoot, "data"),
	}); err != nil {
		return InstallationStatus{}, fmt.Errorf("write native host configuration: %w", err)
	}
	manifestPath := filepath.Join(targetRoot, HostName+".json")
	if err := writeFileJSON(manifestPath, nativeHostManifest{
		Name: HostName, Description: "FluxDM native messaging host", Path: targetHost, Type: "stdio", AllowedOrigins: []string{ExtensionOrigin},
	}); err != nil {
		return InstallationStatus{}, fmt.Errorf("write native host manifest: %w", err)
	}
	if err := registerHost(manifestPath); err != nil {
		return InstallationStatus{}, err
	}
	return InstallationStatus{Ready: true, ExtensionPath: targetExtension, Message: "Browser integration is ready. Use Load unpacked in your browser and select the folder opened below."}, nil
}

func copyExtension(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return os.MkdirAll(destination, 0o700)
		}
		if relative == "README.md" || relative == "policy.test.cjs" || strings.HasPrefix(relative, "native-host"+string(filepath.Separator)) {
			return nil
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unsupported extension entry %q", relative)
		}
		return copyFile(path, target)
	})
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	temporary := destination + ".tmp"
	output, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o700)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		_ = os.Remove(temporary)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(temporary)
		return closeErr
	}
	if err := os.Remove(destination); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(temporary)
		return err
	}
	return os.Rename(temporary, destination)
}

func writeFileJSON(path string, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, payload, 0o600); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(temporary)
		return err
	}
	return os.Rename(temporary, path)
}

func registerHost(manifestPath string) error {
	for _, keyPath := range []string{
		`Software\Google\Chrome\NativeMessagingHosts\` + HostName,
		`Software\Microsoft\Edge\NativeMessagingHosts\` + HostName,
		`Software\BraveSoftware\Brave-Browser\NativeMessagingHosts\` + HostName,
	} {
		key, _, err := registry.CreateKey(registry.CURRENT_USER, keyPath, registry.SET_VALUE)
		if err != nil {
			return fmt.Errorf("register browser native host: %w", err)
		}
		err = key.SetStringValue("", manifestPath)
		closeErr := key.Close()
		if err != nil {
			return fmt.Errorf("register browser native host: %w", err)
		}
		if closeErr != nil {
			return fmt.Errorf("close browser native host registration: %w", closeErr)
		}
	}
	return nil
}
