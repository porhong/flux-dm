package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDesktopExecutablePathPrefersInstalledExecutable(t *testing.T) {
	directory := t.TempDir()
	installed := filepath.Join(directory, "FluxDM.exe")
	if err := os.WriteFile(installed, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "FluxDM-1.0.0-rc.20-windows-amd64-portable.exe"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	actual, err := desktopExecutablePath(directory)
	if err != nil {
		t.Fatal(err)
	}
	if actual != installed {
		t.Fatalf("desktop path = %q, want %q", actual, installed)
	}
}

func TestDesktopExecutablePathFindsVersionedPortableExecutable(t *testing.T) {
	directory := t.TempDir()
	portable := filepath.Join(directory, "FluxDM-1.0.0-rc.20-windows-amd64-portable.exe")
	if err := os.WriteFile(portable, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	actual, err := desktopExecutablePath(directory)
	if err != nil {
		t.Fatal(err)
	}
	if actual != portable {
		t.Fatalf("desktop path = %q, want %q", actual, portable)
	}
}

func TestDesktopExecutablePathRejectsAmbiguousPortableExecutables(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"FluxDM-1.0.0-rc.19-windows-amd64-portable.exe", "FluxDM-1.0.0-rc.20-windows-amd64-portable.exe"} {
		if err := os.WriteFile(filepath.Join(directory, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := desktopExecutablePath(directory); err == nil {
		t.Fatal("expected ambiguous portable executables to be rejected")
	}
}
