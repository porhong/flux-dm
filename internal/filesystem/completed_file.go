package filesystem

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// CompletedFileShell performs explicit Windows shell actions. It is kept
// separate from download I/O so the download engine has no platform coupling.
type CompletedFileShell interface {
	Open(string) error
	Reveal(string) error
	Recycle(string) error
}

// CompletedFileManager safely performs local file operations for completed
// downloads. It deliberately accepts paths rather than download records.
type CompletedFileManager struct{ shell CompletedFileShell }

func NewCompletedFileManager(shell CompletedFileShell) *CompletedFileManager {
	return &CompletedFileManager{shell: shell}
}

func (m *CompletedFileManager) Open(path string) error {
	path, err := validateCompletedFile(path)
	if err != nil {
		return err
	}
	if m.shell == nil {
		return errors.New("file shell is unavailable")
	}
	return m.shell.Open(path)
}

func (m *CompletedFileManager) Reveal(path string) error {
	path, err := validateCompletedFile(path)
	if err != nil {
		return err
	}
	if m.shell == nil {
		return errors.New("file shell is unavailable")
	}
	return m.shell.Reveal(path)
}

func (m *CompletedFileManager) Recycle(path string) error {
	path, err := validateCompletedFile(path)
	if err != nil {
		return err
	}
	if m.shell == nil {
		return errors.New("file shell is unavailable")
	}
	return m.shell.Recycle(path)
}

func (m *CompletedFileManager) Rename(path, name string) (string, error) {
	path, err := validateCompletedFile(path)
	if err != nil {
		return "", err
	}
	return m.move(path, filepath.Dir(path), name)
}

func (m *CompletedFileManager) Move(path, directory string) (string, error) {
	path, err := validateCompletedFile(path)
	if err != nil {
		return "", err
	}
	return m.move(path, directory, filepath.Base(path))
}

func (m *CompletedFileManager) move(source, directory, requestedName string) (string, error) {
	directory, err := ValidateDestinationDirectory(directory)
	if err != nil {
		return "", err
	}
	name := SanitizeFileName(requestedName)
	target, err := availableFilePath(directory, name, source)
	if err != nil {
		return "", err
	}
	if samePath(source, target) {
		return source, nil
	}
	if err := os.Rename(source, target); err == nil {
		return target, nil
	} else if !errors.Is(err, syscall.EXDEV) {
		return "", fmt.Errorf("move completed file: %w", err)
	}
	if err := copyAcrossVolumes(source, target); err != nil {
		return "", err
	}
	return target, nil
}

func validateCompletedFile(path string) (string, error) {
	if path == "" || !filepath.IsAbs(path) {
		return "", errors.New("completed file path must be absolute")
	}
	path = filepath.Clean(path)
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("inspect completed file: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", errors.New("completed file must be a regular file")
	}
	return path, nil
}

func availableFilePath(directory, name, source string) (string, error) {
	extension := filepath.Ext(name)
	base := name[:len(name)-len(extension)]
	for index := 0; index < 10_000; index++ {
		candidateName := name
		if index > 0 {
			candidateName = fmt.Sprintf("%s (%d)%s", base, index, extension)
		}
		candidate := filepath.Join(directory, candidateName)
		if filepath.Dir(candidate) != directory {
			return "", errors.New("filename escapes destination directory")
		}
		if samePath(candidate, source) {
			return candidate, nil
		}
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("inspect move destination: %w", err)
		}
	}
	return "", errors.New("could not find an available filename")
}

func samePath(left, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}

func copyAcrossVolumes(source, target string) error {
	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open completed file: %w", err)
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create moved file: %w", err)
	}
	_, copyErr := io.Copy(output, input)
	syncErr := output.Sync()
	closeErr := output.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(target)
		return fmt.Errorf("copy completed file: %w", errors.Join(copyErr, syncErr, closeErr))
	}
	if err := os.Remove(source); err != nil {
		_ = os.Remove(target)
		return fmt.Errorf("remove original completed file: %w", err)
	}
	return nil
}
