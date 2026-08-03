package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRuntimeUsesInstalledHostConfiguration(t *testing.T) {
	directory := t.TempDir()
	desktop := filepath.Join(directory, "FluxDM-1.0.0-windows-amd64-portable.exe")
	if err := os.WriteFile(desktop, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	host := filepath.Join(directory, "FluxDM.NativeHost.exe")
	if err := os.WriteFile(host, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(runtimeConfig{Version: 1, DesktopPath: desktop, DataDir: filepath.Join(directory, "data")})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "fluxdm-browser-host.json"), payload, 0o600); err != nil {
		t.Fatal(err)
	}
	paths, resolvedDesktop, err := resolveRuntimeForExecutable(host)
	if err != nil {
		t.Fatal(err)
	}
	if paths.DataDir != filepath.Join(directory, "data") || resolvedDesktop != desktop {
		t.Fatalf("runtime = %#v, %q", paths, resolvedDesktop)
	}
}

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
