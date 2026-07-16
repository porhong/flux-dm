package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	invalidWindowsCharacters = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)
	reservedWindowsName      = regexp.MustCompile(`(?i)^(CON|PRN|AUX|NUL|COM[1-9]|LPT[1-9])(?:\..*)?$`)
)

var executableExtensions = map[string]struct{}{".exe": {}, ".msi": {}, ".msp": {}, ".msix": {}, ".appx": {}, ".bat": {}, ".cmd": {}, ".com": {}, ".scr": {}, ".ps1": {}, ".vbs": {}, ".js": {}, ".jse": {}, ".wsf": {}, ".hta": {}, ".cpl": {}, ".reg": {}, ".jar": {}}

const maxFileNameBytes = 240

func SanitizeFileName(name string) string {
	name = strings.ToValidUTF8(name, "_")
	name = strings.TrimSpace(name)
	name = invalidWindowsCharacters.ReplaceAllString(name, "_")
	name = strings.TrimRight(name, ". ")
	if name == "" || name == "." || name == ".." {
		name = "download"
	}
	if reservedWindowsName.MatchString(name) {
		name = "_" + name
	}
	if len(name) > maxFileNameBytes {
		extension := filepath.Ext(name)
		limit := maxFileNameBytes - len(extension)
		if limit < 1 {
			extension = ""
			limit = maxFileNameBytes
		}
		base := strings.TrimSuffix(name, filepath.Ext(name))
		for len(base) > limit {
			_, size := utf8.DecodeLastRuneInString(base)
			base = base[:len(base)-size]
		}
		name = strings.TrimRight(base, ". ") + extension
	}
	return name
}

func IsExecutableLike(name string) bool {
	_, exists := executableExtensions[strings.ToLower(filepath.Ext(strings.TrimSpace(name)))]
	return exists
}

func ValidateDestinationDirectory(path string) (string, error) {
	if path == "" || strings.ContainsRune(path, '\x00') {
		return "", fmt.Errorf("destination directory is required")
	}
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return "", fmt.Errorf("destination directory must be absolute")
	}
	info, err := filepath.EvalSymlinks(clean)
	if err != nil {
		return "", fmt.Errorf("resolve destination directory: %w", err)
	}
	stat, err := os.Stat(info)
	if err != nil {
		return "", fmt.Errorf("inspect destination directory: %w", err)
	}
	if !stat.IsDir() {
		return "", fmt.Errorf("destination path is not a directory")
	}
	resolved := filepath.Clean(info)
	if isSensitiveDestination(resolved) {
		return "", fmt.Errorf("destination is a sensitive Windows directory")
	}
	return resolved, nil
}

func isSensitiveDestination(path string) bool {
	volume := filepath.VolumeName(path)
	if volume != "" && strings.EqualFold(filepath.Clean(path), filepath.Clean(volume+string(os.PathSeparator))) {
		return true
	}
	for _, root := range []string{os.Getenv("SystemRoot"), os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramData")} {
		if root == "" {
			continue
		}
		root = filepath.Clean(root)
		relative, err := filepath.Rel(root, path)
		if err == nil && (relative == "." || (!strings.HasPrefix(relative, ".."+string(os.PathSeparator)) && relative != "..")) {
			return true
		}
	}
	return false
}

func AvailableDestination(directory, requestedName string) (finalPath, tempPath, fileName string, err error) {
	directory, err = ValidateDestinationDirectory(directory)
	if err != nil {
		return "", "", "", err
	}
	fileName = SanitizeFileName(requestedName)
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	extension := filepath.Ext(fileName)
	for index := 0; index < 10_000; index++ {
		candidateName := fileName
		if index > 0 {
			candidateName = fmt.Sprintf("%s (%d)%s", base, index, extension)
		}
		candidate := filepath.Join(directory, candidateName)
		if filepath.Dir(candidate) != directory {
			return "", "", "", fmt.Errorf("filename escapes destination directory")
		}
		part := candidate + ".fluxpart"
		if _, finalErr := os.Stat(candidate); finalErr == nil {
			continue
		} else if !isNotExist(finalErr) {
			return "", "", "", finalErr
		}
		if _, partErr := os.Stat(part); partErr == nil {
			continue
		} else if !isNotExist(partErr) {
			return "", "", "", partErr
		}
		return candidate, part, candidateName, nil
	}
	return "", "", "", fmt.Errorf("could not find an available filename")
}

func ReserveDestination(directory, requestedName string) (finalPath, tempPath, fileName string, err error) {
	for attempts := 0; attempts < 10_000; attempts++ {
		finalPath, tempPath, fileName, err = AvailableDestination(directory, requestedName)
		if err != nil {
			return "", "", "", err
		}
		reservation, openErr := os.OpenFile(tempPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if openErr == nil {
			if closeErr := reservation.Close(); closeErr != nil {
				_ = os.Remove(tempPath)
				return "", "", "", closeErr
			}
			return finalPath, tempPath, fileName, nil
		}
		if os.IsExist(openErr) {
			continue
		}
		return "", "", "", openErr
	}
	return "", "", "", fmt.Errorf("could not reserve an available filename")
}

func isNotExist(err error) bool {
	return os.IsNotExist(err)
}
