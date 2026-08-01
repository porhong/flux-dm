package application

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultDownloadDirectoryForHomeCreatesDownloadsFolder(t *testing.T) {
	home := t.TempDir()
	directory, err := defaultDownloadDirectoryForHome(home)
	if err != nil {
		t.Fatalf("default download directory: %v", err)
	}
	if directory != filepath.Join(home, "Downloads") {
		t.Fatalf("directory = %q, want %q", directory, filepath.Join(home, "Downloads"))
	}
	info, err := os.Stat(directory)
	if err != nil {
		t.Fatalf("stat default download directory: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("default download directory is not a directory")
	}
}

func TestDefaultDownloadDirectoryForHomeRejectsEmptyHome(t *testing.T) {
	if _, err := defaultDownloadDirectoryForHome(" \t"); err == nil {
		t.Fatal("expected empty home directory to be rejected")
	}
}

func TestPortablePathsStoresDataBesideExecutable(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "FluxDM.exe")
	paths, err := portablePaths(executable)
	if err != nil {
		t.Fatal(err)
	}
	if paths.DataDir != filepath.Join(filepath.Dir(executable), "data") {
		t.Fatalf("data directory = %q", paths.DataDir)
	}
}

func TestPortablePathsRejectsRelativeOrEmptyExecutable(t *testing.T) {
	for _, executable := range []string{"", "FluxDM.exe"} {
		if _, err := portablePaths(executable); err == nil {
			t.Fatalf("portablePaths(%q) succeeded", executable)
		}
	}
}

func TestIsPortableBuild(t *testing.T) {
	previous := PortableMode
	t.Cleanup(func() { PortableMode = previous })
	PortableMode = "TrUe"
	if !IsPortableBuild() {
		t.Fatal("portable build not detected")
	}
}
