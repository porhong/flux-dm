package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/fluxdm/fluxdm/internal/application"
	fluxlog "github.com/fluxdm/fluxdm/internal/logging"
	platformwindows "github.com/fluxdm/fluxdm/internal/platform/windows"
	wails "github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:dist
var assets embed.FS

const browserHandoffLaunchArg = "--browser-handoff"

func main() {
	coldBrowserHandoff := len(os.Args) == 2 && os.Args[1] == browserHandoffLaunchArg
	var activator *platformwindows.InstanceActivator
	if !isBindingGeneration() {
		var err error
		activator, err = platformwindows.NewInstanceActivator("Local\\FluxDM.Desktop.Activate")
		if err != nil {
			fmt.Fprintln(os.Stderr, "FluxDM could not initialize its activation signal")
			os.Exit(1)
		}
		defer func() { _ = activator.Close() }()
		instanceLock, err := platformwindows.AcquireInstanceLock("Local\\FluxDM.Desktop.Instance")
		if errors.Is(err, platformwindows.ErrAlreadyRunning) {
			_ = activator.Notify()
			return
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "FluxDM could not secure its single-instance lock")
			os.Exit(1)
		}
		defer func() { _ = instanceLock.Close() }()
	}

	paths, err := application.DefaultPaths()
	if err != nil {
		fmt.Fprintln(os.Stderr, "FluxDM could not determine its data directory")
		os.Exit(1)
	}
	if err := os.MkdirAll(paths.DataDir, 0o700); err != nil {
		fmt.Fprintln(os.Stderr, "FluxDM could not create its data directory")
		os.Exit(1)
	}
	logger, closeLog, err := fluxlog.New(filepath.Join(paths.DataDir, "fluxdm.log"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "FluxDM could not initialize logging")
		os.Exit(1)
	}
	defer closeLog()
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.Error("unhandled application panic", map[string]any{"panic_type": fmt.Sprintf("%T", recovered), "stack": string(debug.Stack())})
		}
	}()

	backend := NewApp(paths, logger)
	desktop := wails.New(wails.Options{
		Name: "FluxDM", Description: "High-performance Windows download manager",
		Windows:  wails.WindowsOptions{DisableQuitOnLastWindowClosed: true},
		Assets:   wails.AssetOptions{Handler: wails.BundledAssetFileServer(assets)},
		Services: []wails.Service{wails.NewService(backend)},
		OnShutdown: func() {
			if activator != nil {
				_ = activator.Close()
			}
			backend.shutdown(context.Background())
		},
	})
	mainWindow := desktop.Window.NewWithOptions(wails.WebviewWindowOptions{Name: "main", Title: "FluxDM", Width: 1180, Height: 760, MinWidth: 840, MinHeight: 600, Hidden: coldBrowserHandoff, URL: "/", BackgroundColour: wails.NewRGB(8, 15, 29)})
	confirmWindow := desktop.Window.NewWithOptions(wails.WebviewWindowOptions{Name: "browser-confirmation", Title: "Start download", Width: 480, Height: 420, MinWidth: 480, MinHeight: 420, MaxWidth: 480, MaxHeight: 420, DisableResize: true, Hidden: true, URL: "/?surface=browser-confirm", BackgroundColour: wails.NewRGB(8, 15, 29)})
	backend.setDesktop(desktop, mainWindow, confirmWindow)
	mainWindow.RegisterHook(events.Common.WindowClosing, func(event *wails.WindowEvent) {
		if backend.beforeClose(context.Background()) {
			event.Cancel()
		}
	})
	confirmWindow.RegisterHook(events.Common.WindowClosing, func(event *wails.WindowEvent) { confirmWindow.Hide(); event.Cancel() })
	if activator != nil {
		if err := activator.Start(backend.showWindow); err != nil {
			logger.Error("instance activation listener failed to start", map[string]any{"error": err.Error()})
		}
	}
	if err := desktop.Run(); err != nil {
		logger.Error("application stopped unexpectedly", map[string]any{"error": err.Error()})
	}
}
