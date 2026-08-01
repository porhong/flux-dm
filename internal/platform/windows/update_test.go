package windows

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fluxdm/fluxdm/internal/update"
)

func TestUpdateLauncherStartsDetachedHelper(t *testing.T) {
	directory := t.TempDir()
	helper := filepath.Join(directory, "FluxDM.UpdateLauncher.exe")
	installer := filepath.Join(directory, "installer.exe")
	restart := filepath.Join(directory, "FluxDM.exe")
	for _, path := range []string{helper, installer, restart} {
		if err := os.WriteFile(path, []byte("test executable"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	resultPath := filepath.Join(directory, "launcher", "result.json")
	launcher := UpdateLauncher{HelperPath: helper, RestartPath: restart, CacheDir: filepath.Join(directory, "cache")}
	handoff := update.Handoff{TargetVersion: "1.0.0-rc.16", Token: strings.Repeat("a", 64), ResultPath: resultPath}

	originalStart := startUpdateHelper
	t.Cleanup(func() { startUpdateHelper = originalStart })
	var got *exec.Cmd
	startUpdateHelper = func(command *exec.Cmd) error {
		got = command
		return nil
	}
	if err := launcher.Launch(context.Background(), installer, handoff); err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("update helper was not started")
	}
	if got.Cancel != nil {
		t.Fatal("update helper is coupled to its caller context")
	}
	wantPath := filepath.Join(launcher.CacheDir, "FluxDM.UpdateLauncher.exe")
	if got.Path != wantPath {
		t.Fatalf("helper path = %q, want %q", got.Path, wantPath)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("cached helper missing: %v", err)
	}
	wantArgs := []string{"-parent-pid", "-installer", installer, "-restart", restart, "-target-version", handoff.TargetVersion, "-token", handoff.Token, "-result", resultPath}
	for _, want := range wantArgs {
		if !contains(got.Args, want) {
			t.Fatalf("helper arguments %q do not contain %q", got.Args, want)
		}
	}
}

func TestUpdateLauncherRejectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	launcher := UpdateLauncher{}
	if err := launcher.Launch(ctx, "", update.Handoff{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", err)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
