package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fluxdm/fluxdm/internal/application"
	"github.com/fluxdm/fluxdm/internal/browserintegration"
)

func main() {
	if len(os.Args) < 2 || browserintegration.ValidateOrigin(os.Args[1]) != nil {
		fmt.Fprintln(os.Stderr, "FluxDM browser host rejected the caller")
		os.Exit(2)
	}
	paths, desktop, err := resolveRuntime()
	if err != nil {
		os.Exit(1)
	}
	launched := false
	for {
		request, readErr := browserintegration.ReadMessage(os.Stdin)
		if errors.Is(readErr, io.EOF) {
			return
		}
		if readErr != nil {
			_ = browserintegration.WriteMessage(os.Stdout, browserintegration.Response{Version: 1, Accepted: false, Code: "invalid_message", Message: "The native request was invalid."})
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		response, forwardErr := browserintegration.Forward(ctx, paths.DataDir, request)
		if forwardErr != nil && !launched {
			if launchDesktop(desktop) == nil {
				launched = true
				for attempt := 0; attempt < 40 && forwardErr != nil; attempt++ {
					select {
					case <-ctx.Done():
						break
					case <-time.After(125 * time.Millisecond):
						response, forwardErr = browserintegration.Forward(ctx, paths.DataDir, request)
					}
				}
			}
		}
		cancel()
		if forwardErr != nil {
			response = browserintegration.Response{Version: 1, RequestID: request.RequestID, Accepted: false, Code: "desktop_unavailable", Message: "Open FluxDM and try again."}
		}
		if err := browserintegration.WriteMessage(os.Stdout, response); err != nil {
			return
		}
	}
}

func launchDesktop(desktop string) error {
	if strings.TrimSpace(desktop) == "" {
		return errors.New("desktop executable is required")
	}
	// This fixed internal argument is the only launch mode that suppresses the
	// dashboard. It is never built from browser-provided input.
	command := exec.Command(desktop, "--browser-handoff")
	command.Stdin = nil
	command.Stdout = nil
	command.Stderr = nil
	return command.Start()
}

type runtimeConfig struct {
	Version     int    `json:"version"`
	DesktopPath string `json:"desktopPath"`
	DataDir     string `json:"dataDir"`
}

func resolveRuntime() (application.Paths, string, error) {
	executable, err := os.Executable()
	if err != nil {
		return application.Paths{}, "", err
	}
	return resolveRuntimeForExecutable(executable)
}

func resolveRuntimeForExecutable(executable string) (application.Paths, string, error) {
	configPath := filepath.Join(filepath.Dir(executable), "fluxdm-browser-host.json")
	payload, err := os.ReadFile(configPath)
	if errors.Is(err, os.ErrNotExist) {
		paths, pathErr := application.DefaultPaths()
		if pathErr != nil {
			return application.Paths{}, "", pathErr
		}
		desktop, desktopErr := desktopExecutablePath(filepath.Dir(executable))
		return paths, desktop, desktopErr
	}
	if err != nil {
		return application.Paths{}, "", err
	}
	if len(payload) > 4096 {
		return application.Paths{}, "", errors.New("native host configuration is too large")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var config runtimeConfig
	if err := decoder.Decode(&config); err != nil {
		return application.Paths{}, "", err
	}
	if config.Version != 1 || !filepath.IsAbs(config.DesktopPath) || !filepath.IsAbs(config.DataDir) {
		return application.Paths{}, "", errors.New("native host configuration is invalid")
	}
	if filepath.Clean(config.DataDir) != filepath.Join(filepath.Dir(config.DesktopPath), "data") {
		return application.Paths{}, "", errors.New("native host data directory is invalid")
	}
	if info, err := os.Stat(config.DesktopPath); err != nil || info.IsDir() {
		return application.Paths{}, "", errors.New("configured FluxDM desktop executable is unavailable")
	}
	return application.Paths{DataDir: config.DataDir}, config.DesktopPath, nil
}

func desktopExecutablePath(directory string) (string, error) {
	installed := filepath.Join(directory, "FluxDM.exe")
	if info, err := os.Stat(installed); err == nil && !info.IsDir() {
		return installed, nil
	}
	portable, err := filepath.Glob(filepath.Join(directory, "FluxDM-*-windows-amd64-portable.exe"))
	if err != nil {
		return "", err
	}
	if len(portable) != 1 {
		return "", errors.New("portable FluxDM executable was not found uniquely")
	}
	return portable[0], nil
}
