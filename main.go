package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/fluxdm/fluxdm/internal/application"
	fluxlog "github.com/fluxdm/fluxdm/internal/logging"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:dist
var assets embed.FS

func main() {
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

	app := NewApp(paths, logger)
	if err := wails.Run(&options.App{
		Title:     "FluxDM",
		Width:     1180,
		Height:    760,
		MinWidth:  840,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 8, G: 15, B: 29, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		OnBeforeClose:    app.beforeClose,
		Bind:             []interface{}{app},
	}); err != nil {
		logger.Error("application stopped unexpectedly", map[string]any{"error": err.Error()})
	}
}
