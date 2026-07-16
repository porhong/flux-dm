package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeFileName(t *testing.T) {
	tests := map[string]string{
		"report?.pdf": "report_.pdf",
		"CON":         "_CON",
		"trailing. ":  "trailing",
		"../../":      ".._.._",
		"":            "download",
	}
	for input, expected := range tests {
		if actual := SanitizeFileName(input); actual != expected {
			t.Errorf("SanitizeFileName(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestExecutableLikeWarning(t *testing.T) {
	for _, name := range []string{"setup.EXE", "script.ps1", "package.msi"} {
		if !IsExecutableLike(name) {
			t.Fatalf("expected warning for %s", name)
		}
	}
	if IsExecutableLike("archive.zip") {
		t.Fatal("warned for archive")
	}
}

func TestAvailableDestinationAvoidsDuplicates(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "archive.zip"), []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	finalPath, tempPath, fileName, err := AvailableDestination(directory, "archive.zip")
	if err != nil {
		t.Fatal(err)
	}
	if fileName != "archive (1).zip" || finalPath != filepath.Join(directory, fileName) || tempPath != finalPath+".fluxpart" {
		t.Fatalf("unexpected destination: %q %q %q", finalPath, tempPath, fileName)
	}
}

func TestReserveDestinationPreventsQueuedNameCollisions(t *testing.T) {
	directory := t.TempDir()
	_, firstTemp, firstName, err := ReserveDestination(directory, "video.mp4")
	if err != nil {
		t.Fatal(err)
	}
	_, secondTemp, secondName, err := ReserveDestination(directory, "video.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if firstName != "video.mp4" || secondName != "video (1).mp4" || firstTemp == secondTemp {
		t.Fatalf("reservations collided: %q %q %q %q", firstName, secondName, firstTemp, secondTemp)
	}
}

func TestValidateDestinationRejectsInvalidPaths(t *testing.T) {
	if _, err := ValidateDestinationDirectory("relative/path"); err == nil {
		t.Fatal("expected relative path to be rejected")
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateDestinationDirectory(file); err == nil {
		t.Fatal("expected file destination to be rejected")
	}
}
