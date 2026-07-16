package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/fluxdm/fluxdm/internal/application"
	"github.com/fluxdm/fluxdm/internal/browserintegration"
)

func main() {
	if len(os.Args) < 2 || browserintegration.ValidateOrigin(os.Args[1]) != nil {
		fmt.Fprintln(os.Stderr, "FluxDM browser host rejected the caller")
		os.Exit(2)
	}
	paths, err := application.DefaultPaths()
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
			if launchDesktop() == nil {
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

func launchDesktop() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	desktop := filepath.Join(filepath.Dir(executable), "FluxDM.exe")
	if _, err := os.Stat(desktop); err != nil {
		return err
	}
	command := exec.Command(desktop)
	command.Stdin = nil
	command.Stdout = nil
	command.Stderr = nil
	return command.Start()
}
