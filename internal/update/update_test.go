package update

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type memoryRepository struct {
	mu    sync.Mutex
	state StoredState
}

func (r *memoryRepository) Load(context.Context) (StoredState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state, nil
}
func (r *memoryRepository) Save(_ context.Context, state StoredState) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state = state
	return nil
}

type acceptingVerifier struct{ calls int }

func (v *acceptingVerifier) VerifyProductionInstaller(string) error { v.calls++; return nil }

type recordingLauncher struct{ path string }

func (l *recordingLauncher) Launch(_ context.Context, path string) error { l.path = path; return nil }

func TestManagerChecksDownloadsAndInstallsSignedStableRelease(t *testing.T) {
	stablePublic, stablePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	previewPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	installer := []byte("verified installer bytes")
	updateManifest := manifest{Version: "1.1.0", ProductVersion: "1.1.0", Channel: ChannelStable, Signed: true, ReleaseNotesURL: "https://example.test/notes"}
	updateManifest.Installer.File = "FluxDM-1.1.0-windows-amd64-installer.exe"
	updateManifest.Installer.SHA256 = sha256Hex(installer)
	updateManifest.Installer.Bytes = int64(len(installer))
	payload, err := json.Marshal(updateManifest)
	if err != nil {
		t.Fatal(err)
	}
	signature := []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(stablePrivate, payload)))
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/releases":
			_ = json.NewEncoder(response).Encode([]githubRelease{{TagName: "v1.1.0", Assets: []githubAsset{{Name: "update-manifest.json", BrowserDownloadURL: server.URL + "/manifest"}, {Name: "update-manifest.sig", BrowserDownloadURL: server.URL + "/signature"}, {Name: updateManifest.Installer.File, BrowserDownloadURL: server.URL + "/installer"}}}})
		case "/manifest":
			_, _ = response.Write(payload)
		case "/signature":
			_, _ = response.Write(signature)
		case "/installer":
			_, _ = response.Write(installer)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()
	repository := &memoryRepository{state: StoredState{Channel: ChannelStable, AutoDownload: true, Phase: PhaseIdle}}
	verifier := &acceptingVerifier{}
	launcher := &recordingLauncher{}
	manager, err := NewManager(Config{Repository: "test/repo", ReleaseAPIURL: server.URL + "/releases", CacheDir: t.TempDir(), CurrentVersion: "1.0.0", StablePublicKey: stablePublic, PreviewPublicKey: previewPublic, HTTPClient: server.Client(), Verifier: verifier, Launcher: launcher}, repository)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	status, err := manager.Check(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if status.Phase != PhaseReady || !status.CanInstall || status.AvailableVersion != "1.1.0" {
		t.Fatalf("unexpected status: %#v", status)
	}
	if verifier.calls != 1 {
		t.Fatalf("signature verifier calls=%d", verifier.calls)
	}
	if _, err := os.Stat(filepath.Join(manager.config.CacheDir, "1.1.0", "installer.exe")); err != nil {
		t.Fatal(err)
	}
	if err := manager.Install(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if launcher.path == "" {
		t.Fatal("launcher did not receive verified path")
	}
}

func TestPreviewInstallationRequiresConfirmation(t *testing.T) {
	manager := testManager(t)
	manager.state = StoredState{Channel: ChannelPreview, Phase: PhaseReady, InstallerPath: filepath.Join(t.TempDir(), "installer.exe")}
	if err := os.WriteFile(manager.state.InstallerPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := manager.Install(context.Background(), false); err == nil {
		t.Fatal("expected preview confirmation error")
	}
}
func TestCompareVersionsOrdersRCBeforeFinal(t *testing.T) {
	if compareVersions("1.0.0-rc.9", "1.0.0") >= 0 {
		t.Fatal("rc must be older than final")
	}
	if compareVersions("1.0.1", "1.0.0") <= 0 {
		t.Fatal("patch must be newer")
	}
}
func testManager(t *testing.T) *Manager {
	t.Helper()
	stable, _, _ := ed25519.GenerateKey(rand.Reader)
	preview, _, _ := ed25519.GenerateKey(rand.Reader)
	value, err := NewManager(Config{Repository: "test/repo", CacheDir: t.TempDir(), CurrentVersion: "1.0.0", StablePublicKey: stable, PreviewPublicKey: preview, HTTPClient: http.DefaultClient, Verifier: &acceptingVerifier{}, Launcher: &recordingLauncher{}}, &memoryRepository{})
	if err != nil {
		t.Fatal(err)
	}
	return value
}
func sha256Hex(value []byte) string { hash := sha256.Sum256(value); return fmt.Sprintf("%x", hash[:]) }
