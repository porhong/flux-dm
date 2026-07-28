// Package update owns secure update discovery, download, and installation
// preparation. It deliberately has no dependency on Wails or download-engine
// packages.
package update

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	ChannelStable           = "stable"
	ChannelPreview          = "preview"
	PhaseIdle               = "idle"
	PhaseChecking           = "checking"
	PhaseAvailable          = "available"
	PhaseDownloading        = "downloading"
	PhaseReady              = "ready"
	PhaseInstalling         = "installing"
	PhaseError              = "error"
	maxManifestBytes  int64 = 128 << 10
	maxInstallerBytes int64 = 2 << 30
)

// ReleaseVersion is set by release builds. Version is the numeric product
// version reported by Wails; ReleaseVersion retains an RC suffix when present.
var ReleaseVersion = "1.0.0"

type StoredState struct {
	Channel          string
	AutoDownload     bool
	Phase            string
	AvailableVersion string
	ReleaseNotesURL  string
	InstallerPath    string
	InstallerSHA256  string
	DownloadedBytes  int64
	TotalBytes       int64
	LastCheckedAt    string
	LastError        string
}

type Repository interface {
	Load(context.Context) (StoredState, error)
	Save(context.Context, StoredState) error
}

type Verifier interface{ VerifyProductionInstaller(path string) error }
type Launcher interface {
	Launch(ctx context.Context, installerPath string) error
}
type Notifier func(Status)

type Config struct {
	Repository       string
	ReleaseAPIURL    string // test-only override; production uses the pinned repository endpoint.
	CacheDir         string
	CurrentVersion   string
	StablePublicKey  ed25519.PublicKey
	PreviewPublicKey ed25519.PublicKey
	HTTPClient       *http.Client
	Verifier         Verifier
	Launcher         Launcher
}

type Preferences struct {
	Channel      string
	AutoDownload bool
}
type Status struct {
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
}

type manifest struct {
	Version         string `json:"version"`
	ProductVersion  string `json:"productVersion"`
	Channel         string `json:"channel"`
	Signed          bool   `json:"signed"`
	MinimumVersion  string `json:"minimumVersion"`
	ReleaseNotesURL string `json:"releaseNotesUrl"`
	Installer       struct {
		File   string `json:"file"`
		SHA256 string `json:"sha256"`
		Bytes  int64  `json:"bytes"`
	} `json:"installer"`
}
type githubRelease struct {
	TagName    string        `json:"tag_name"`
	Draft      bool          `json:"draft"`
	Prerelease bool          `json:"prerelease"`
	Assets     []githubAsset `json:"assets"`
}
type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type Manager struct {
	config     Config
	repository Repository
	mu         sync.Mutex
	state      StoredState
	notify     Notifier
	cancel     context.CancelFunc
}

func NewManager(config Config, repository Repository) (*Manager, error) {
	if strings.TrimSpace(config.Repository) == "" || strings.TrimSpace(config.CacheDir) == "" || !validVersion(config.CurrentVersion) {
		return nil, errors.New("invalid update configuration")
	}
	if config.HTTPClient == nil || config.Verifier == nil || config.Launcher == nil || len(config.StablePublicKey) != ed25519.PublicKeySize || len(config.PreviewPublicKey) != ed25519.PublicKeySize {
		return nil, errors.New("incomplete update configuration")
	}
	return &Manager{config: config, repository: repository}, nil
}

func (m *Manager) SetNotifier(notify Notifier) { m.mu.Lock(); m.notify = notify; m.mu.Unlock() }
func (m *Manager) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	m.mu.Lock()
	if m.cancel != nil {
		m.mu.Unlock()
		cancel()
		return
	}
	m.cancel = cancel
	m.mu.Unlock()
	go func() {
		timer := time.NewTimer(90 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				_, _ = m.Check(ctx, false)
				timer.Reset(24 * time.Hour)
			}
		}
	}()
}
func (m *Manager) Close() {
	m.mu.Lock()
	cancel := m.cancel
	m.cancel = nil
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (m *Manager) Load(ctx context.Context) (Status, error) {
	state, err := m.repository.Load(ctx)
	if err != nil {
		return Status{}, err
	}
	m.mu.Lock()
	m.state = state
	status := m.statusLocked()
	m.mu.Unlock()
	return status, nil
}
func (m *Manager) SavePreferences(ctx context.Context, preferences Preferences) (Status, error) {
	if preferences.Channel != ChannelStable && preferences.Channel != ChannelPreview {
		return Status{}, errors.New("unsupported update channel")
	}
	m.mu.Lock()
	if m.state.Channel != "" && m.state.Channel != preferences.Channel {
		m.state = StoredState{Channel: preferences.Channel, AutoDownload: preferences.AutoDownload, Phase: PhaseIdle}
	} else {
		m.state.Channel = preferences.Channel
		m.state.AutoDownload = preferences.AutoDownload
	}
	state := m.state
	m.mu.Unlock()
	if err := m.repository.Save(ctx, state); err != nil {
		return Status{}, err
	}
	m.emit()
	return m.Status(), nil
}
func (m *Manager) Status() Status { m.mu.Lock(); defer m.mu.Unlock(); return m.statusLocked() }
func (m *Manager) statusLocked() Status {
	return Status{CurrentVersion: m.config.CurrentVersion, Channel: defaultChannel(m.state.Channel), AutoDownload: m.state.AutoDownload, Phase: defaultPhase(m.state.Phase), AvailableVersion: m.state.AvailableVersion, ReleaseNotesURL: m.state.ReleaseNotesURL, DownloadedBytes: m.state.DownloadedBytes, TotalBytes: m.state.TotalBytes, LastCheckedAt: m.state.LastCheckedAt, LastError: m.state.LastError, Preview: m.state.Channel == ChannelPreview, CanInstall: m.state.Phase == PhaseReady && m.state.InstallerPath != ""}
}
func (m *Manager) emit() {
	m.mu.Lock()
	notify := m.notify
	status := m.statusLocked()
	m.mu.Unlock()
	if notify != nil {
		notify(status)
	}
}

func (m *Manager) Check(ctx context.Context, autoDownload bool) (Status, error) {
	m.setPhase(ctx, PhaseChecking, "")
	release, err := m.findRelease(ctx)
	if err != nil {
		return m.fail(ctx, err)
	}
	if release == nil {
		m.mu.Lock()
		m.state.Phase = PhaseIdle
		m.state.LastCheckedAt = time.Now().UTC().Format(time.RFC3339Nano)
		m.state.LastError = ""
		state := m.state
		m.mu.Unlock()
		_ = m.repository.Save(ctx, state)
		m.emit()
		return m.Status(), nil
	}
	manifestPayload, signature, err := m.fetchManifest(ctx, *release)
	if err != nil {
		return m.fail(ctx, err)
	}
	parsed, err := m.verifyManifest(manifestPayload, signature, release.Prerelease)
	if err != nil {
		return m.fail(ctx, err)
	}
	if compareVersions(parsed.Version, m.config.CurrentVersion) <= 0 {
		m.mu.Lock()
		m.state.Phase = PhaseIdle
		m.state.LastCheckedAt = time.Now().UTC().Format(time.RFC3339Nano)
		m.state.LastError = ""
		state := m.state
		m.mu.Unlock()
		_ = m.repository.Save(ctx, state)
		m.emit()
		return m.Status(), nil
	}
	m.mu.Lock()
	m.state.Phase = PhaseAvailable
	m.state.AvailableVersion = parsed.Version
	m.state.ReleaseNotesURL = parsed.ReleaseNotesURL
	m.state.InstallerSHA256 = parsed.Installer.SHA256
	m.state.TotalBytes = parsed.Installer.Bytes
	m.state.DownloadedBytes = 0
	m.state.LastCheckedAt = time.Now().UTC().Format(time.RFC3339Nano)
	m.state.LastError = ""
	state := m.state
	m.mu.Unlock()
	if err := m.repository.Save(ctx, state); err != nil {
		return Status{}, err
	}
	m.emit()
	if autoDownload && parsed.Channel == ChannelStable && m.Status().AutoDownload {
		return m.Download(ctx, *release, parsed)
	}
	return m.Status(), nil
}

func (m *Manager) DownloadAvailable(ctx context.Context) (Status, error) {
	release, err := m.findRelease(ctx)
	if err != nil {
		return m.fail(ctx, err)
	}
	if release == nil {
		return Status{}, errors.New("no update release is available")
	}
	payload, sig, err := m.fetchManifest(ctx, *release)
	if err != nil {
		return m.fail(ctx, err)
	}
	parsed, err := m.verifyManifest(payload, sig, release.Prerelease)
	if err != nil {
		return m.fail(ctx, err)
	}
	return m.Download(ctx, *release, parsed)
}
func (m *Manager) Download(ctx context.Context, release githubRelease, parsed manifest) (Status, error) {
	if parsed.Version != m.Status().AvailableVersion {
		return Status{}, errors.New("update availability changed; check again")
	}
	asset, ok := findAsset(release.Assets, parsed.Installer.File)
	if !ok {
		return m.fail(ctx, errors.New("release installer is missing"))
	}
	if parsed.Installer.Bytes <= 0 || parsed.Installer.Bytes > maxInstallerBytes || len(parsed.Installer.SHA256) != 64 || filepath.Base(parsed.Installer.File) != parsed.Installer.File || !strings.HasSuffix(strings.ToLower(parsed.Installer.File), ".exe") {
		return m.fail(ctx, errors.New("invalid installer metadata"))
	}
	if _, err := hex.DecodeString(parsed.Installer.SHA256); err != nil {
		return m.fail(ctx, errors.New("invalid installer checksum"))
	}
	m.setPhase(ctx, PhaseDownloading, "")
	dir := filepath.Join(m.config.CacheDir, parsed.Version)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return m.fail(ctx, err)
	}
	final := filepath.Join(dir, "installer.exe")
	partial := final + ".part"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return m.fail(ctx, err)
	}
	response, err := m.config.HTTPClient.Do(request)
	if err != nil {
		return m.fail(ctx, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return m.fail(ctx, fmt.Errorf("download installer: %s", response.Status))
	}
	if response.ContentLength > maxInstallerBytes {
		return m.fail(ctx, errors.New("installer is too large"))
	}
	file, err := os.OpenFile(partial, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return m.fail(ctx, err)
	}
	hash := sha256.New()
	writer := io.MultiWriter(file, hash)
	written, copyErr := io.Copy(writer, io.LimitReader(response.Body, maxInstallerBytes+1))
	closeErr := file.Close()
	if copyErr != nil {
		return m.fail(ctx, copyErr)
	}
	if closeErr != nil {
		return m.fail(ctx, closeErr)
	}
	if written != parsed.Installer.Bytes || written > maxInstallerBytes || !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), parsed.Installer.SHA256) {
		_ = os.Remove(partial)
		return m.fail(ctx, errors.New("installer checksum verification failed"))
	}
	if err := os.Rename(partial, final); err != nil {
		return m.fail(ctx, err)
	}
	if parsed.Signed {
		if err := m.config.Verifier.VerifyProductionInstaller(final); err != nil {
			_ = os.Remove(final)
			return m.fail(ctx, fmt.Errorf("verify production signature: %w", err))
		}
	}
	m.mu.Lock()
	m.state.Phase = PhaseReady
	m.state.InstallerPath = final
	m.state.DownloadedBytes = written
	m.state.TotalBytes = written
	state := m.state
	m.mu.Unlock()
	if err := m.repository.Save(ctx, state); err != nil {
		return Status{}, err
	}
	m.emit()
	return m.Status(), nil
}
func (m *Manager) Install(ctx context.Context, confirmPreview bool) error {
	m.mu.Lock()
	state := m.state
	m.mu.Unlock()
	if state.Phase != PhaseReady || state.InstallerPath == "" {
		return errors.New("no verified update is ready")
	}
	if state.Channel == ChannelPreview && !confirmPreview {
		return errors.New("preview installation requires confirmation")
	}
	if _, err := os.Stat(state.InstallerPath); err != nil {
		return errors.New("verified installer is no longer available")
	}
	m.setPhase(ctx, PhaseInstalling, "")
	return m.config.Launcher.Launch(ctx, state.InstallerPath)
}
func (m *Manager) setPhase(ctx context.Context, phase, message string) {
	m.mu.Lock()
	m.state.Phase = phase
	m.state.LastError = message
	state := m.state
	m.mu.Unlock()
	_ = m.repository.Save(ctx, state)
	m.emit()
}
func (m *Manager) fail(ctx context.Context, err error) (Status, error) {
	m.setPhase(ctx, PhaseError, err.Error())
	return m.Status(), err
}

func (m *Manager) findRelease(ctx context.Context) (*githubRelease, error) {
	endpoint := m.config.ReleaseAPIURL
	if endpoint == "" {
		endpoint = "https://api.github.com/repos/" + m.config.Repository + "/releases?per_page=100"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	response, err := m.config.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("check releases: %s", response.Status)
	}
	var releases []githubRelease
	if err := json.NewDecoder(io.LimitReader(response.Body, maxManifestBytes)).Decode(&releases); err != nil {
		return nil, err
	}
	var selected *githubRelease
	channel := m.Status().Channel
	for i := range releases {
		candidate := &releases[i]
		if candidate.Draft {
			continue
		}
		if channel == ChannelStable && candidate.Prerelease {
			continue
		}
		if channel == ChannelPreview && candidate.Prerelease == false { /* stable is eligible too */
		}
		if !validVersion(strings.TrimPrefix(candidate.TagName, "v")) {
			continue
		}
		if selected == nil || compareVersions(strings.TrimPrefix(candidate.TagName, "v"), strings.TrimPrefix(selected.TagName, "v")) > 0 {
			selected = candidate
		}
	}
	return selected, nil
}
func (m *Manager) fetchManifest(ctx context.Context, release githubRelease) ([]byte, []byte, error) {
	manifestAsset, ok := findAsset(release.Assets, "update-manifest.json")
	if !ok {
		return nil, nil, errors.New("release update manifest is missing")
	}
	sigAsset, ok := findAsset(release.Assets, "update-manifest.sig")
	if !ok {
		return nil, nil, errors.New("release update signature is missing")
	}
	payload, err := m.fetchSmall(ctx, manifestAsset.BrowserDownloadURL)
	if err != nil {
		return nil, nil, err
	}
	sigText, err := m.fetchSmall(ctx, sigAsset.BrowserDownloadURL)
	if err != nil {
		return nil, nil, err
	}
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigText)))
	return payload, signature, err
}
func (m *Manager) fetchSmall(ctx context.Context, rawURL string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := m.config.HTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download metadata: %s", response.Status)
	}
	value, err := io.ReadAll(io.LimitReader(response.Body, maxManifestBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(value)) > maxManifestBytes {
		return nil, errors.New("update metadata is too large")
	}
	return value, nil
}
func (m *Manager) verifyManifest(payload, signature []byte, prerelease bool) (manifest, error) {
	if int64(len(payload)) > maxManifestBytes || len(signature) != ed25519.SignatureSize {
		return manifest{}, errors.New("invalid update signature")
	}
	key := m.config.StablePublicKey
	if prerelease {
		key = m.config.PreviewPublicKey
	}
	if !ed25519.Verify(key, payload, signature) {
		return manifest{}, errors.New("update metadata signature is invalid")
	}
	var parsed manifest
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return parsed, err
	}
	if !validVersion(parsed.Version) || parsed.ProductVersion == "" || (parsed.MinimumVersion != "" && !validVersion(parsed.MinimumVersion)) || (parsed.ReleaseNotesURL != "" && !isHTTPSURL(parsed.ReleaseNotesURL)) || (parsed.Channel != ChannelStable && parsed.Channel != ChannelPreview) || parsed.Channel == ChannelStable && prerelease || parsed.Channel == ChannelPreview && !prerelease || parsed.Signed != (!prerelease) {
		return parsed, errors.New("update metadata does not match release channel")
	}
	return parsed, nil
}
func findAsset(assets []githubAsset, name string) (githubAsset, bool) {
	for _, asset := range assets {
		if asset.Name == name {
			parsed, err := url.Parse(asset.BrowserDownloadURL)
			if err == nil && parsed.Scheme == "https" {
				return asset, true
			}
		}
	}
	return githubAsset{}, false
}
func isHTTPSURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}
func defaultChannel(value string) string {
	if value == ChannelPreview {
		return value
	}
	return ChannelStable
}
func defaultPhase(value string) string {
	if value == "" {
		return PhaseIdle
	}
	return value
}
func validVersion(value string) bool { _, ok := parseVersion(value); return ok }

type version struct {
	major, minor, patch int
	rc                  int
	prerelease          bool
}

func parseVersion(value string) (version, bool) {
	value = strings.TrimPrefix(value, "v")
	var parsed version
	core := value
	if index := strings.Index(value, "-rc."); index >= 0 {
		core = value[:index]
		suffix := value[index+4:]
		if suffix == "" {
			return parsed, false
		}
		for _, r := range suffix {
			if r < '0' || r > '9' {
				return parsed, false
			}
		}
		_, err := fmt.Sscanf(suffix, "%d", &parsed.rc)
		if err != nil || parsed.rc < 1 {
			return parsed, false
		}
		parsed.prerelease = true
	} else if strings.Contains(value, "-") {
		return parsed, false
	}
	if _, err := fmt.Sscanf(core, "%d.%d.%d", &parsed.major, &parsed.minor, &parsed.patch); err != nil {
		return parsed, false
	}
	if fmt.Sprintf("%d.%d.%d", parsed.major, parsed.minor, parsed.patch) != core {
		return parsed, false
	}
	return parsed, true
}
func compareVersions(left, right string) int {
	a, aok := parseVersion(left)
	b, bok := parseVersion(right)
	if !aok || !bok {
		return 0
	}
	for _, pair := range [][2]int{{a.major, b.major}, {a.minor, b.minor}, {a.patch, b.patch}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	if a.prerelease != b.prerelease {
		if a.prerelease {
			return -1
		}
		return 1
	}
	if a.rc < b.rc {
		return -1
	}
	if a.rc > b.rc {
		return 1
	}
	return 0
}
