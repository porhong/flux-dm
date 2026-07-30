package update

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

// Handoff carries the information the private update helper needs to finish
// an installation. Token is an unguessable correlation value and is never
// displayed or logged.
type Handoff struct {
	TargetVersion string
	Token         string
	ResultPath    string
}

type HandoffResult struct {
	TargetVersion string `json:"targetVersion"`
	Token         string `json:"token"`
	Succeeded     bool   `json:"succeeded"`
	Failure       string `json:"failure,omitempty"`
}

func ReadHandoffResult(path string) (HandoffResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return HandoffResult{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 4097))
	var result HandoffResult
	if err := decoder.Decode(&result); err != nil {
		return HandoffResult{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return HandoffResult{}, errors.New("invalid update handoff result")
	}
	if !validVersion(result.TargetVersion) || !validToken(result.Token) || (result.Succeeded && result.Failure != "") || (!result.Succeeded && !validFailure(result.Failure)) {
		return HandoffResult{}, errors.New("invalid update handoff result")
	}
	return result, nil
}

// WriteHandoffResult replaces the result in one rename, so a restarted app
// never observes a partial helper result.
func WriteHandoffResult(path string, result HandoffResult) error {
	if !filepath.IsAbs(path) || !validVersion(result.TargetVersion) || !validToken(result.Token) || (result.Succeeded && result.Failure != "") || (!result.Succeeded && !validFailure(result.Failure)) {
		return errors.New("invalid update handoff result")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".handoff-*.tmp")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(payload); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, path)
}

func validToken(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

func validFailure(value string) bool {
	return value == "parent_exit_timeout" || value == "installer_failed" || value == "restart_failed"
}

func safeHandoffFailure(value string) string {
	switch value {
	case "parent_exit_timeout":
		return "FluxDM did not close in time. Retry restart and install."
	case "installer_failed":
		return "The update installer did not complete. Retry restart and install."
	case "restart_failed":
		return "The update was installed, but FluxDM could not restart. Retry restart and install."
	default:
		return "The update installation could not be confirmed. Retry restart and install."
	}
}
