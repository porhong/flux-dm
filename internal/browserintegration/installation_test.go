package browserintegration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallPortableRejectsIncompleteBundleWithoutRegistration(t *testing.T) {
	directory := t.TempDir()
	desktop := filepath.Join(directory, "FluxDM.exe")
	if err := os.WriteFile(desktop, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LOCALAPPDATA", t.TempDir())
	status, err := InstallPortable(desktop, "1.0.0")
	if err == nil {
		t.Fatal("expected incomplete portable bundle to be rejected")
	}
	if status.Ready || status.Message == "" {
		t.Fatalf("unexpected status: %#v", status)
	}
}

func TestInstallPortableRejectsUnsafeVersion(t *testing.T) {
	status, err := InstallPortable(`C:\portable\FluxDM.exe`, `../1.0.0`)
	if err == nil || status.Ready {
		t.Fatalf("unexpected status=%#v err=%v", status, err)
	}
}
