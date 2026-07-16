package main

import (
	_ "embed"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed build/windows/icon.ico
var trayIcon []byte

func (a *App) startTray() {
	if a.trayStarted.Swap(true) {
		return
	}
	go systray.Run(func() {
		systray.SetIcon(trayIcon)
		systray.SetTitle("FluxDM")
		systray.SetTooltip("FluxDM download manager")
		show := systray.AddMenuItem("Show FluxDM", "Show the main window")
		add := systray.AddMenuItem("Add download", "Open the Add Download dialog")
		systray.AddSeparator()
		exit := systray.AddMenuItem("Exit", "Exit FluxDM")
		go func() {
			for {
				select {
				case <-a.ctx.Done():
					return
				case <-show.ClickedCh:
					runtime.WindowShow(a.ctx)
					runtime.WindowUnminimise(a.ctx)
				}
			}
		}()
		go func() {
			for {
				select {
				case <-a.ctx.Done():
					return
				case <-add.ClickedCh:
					runtime.WindowShow(a.ctx)
					runtime.WindowUnminimise(a.ctx)
					runtime.EventsEmit(a.ctx, "tray:add-download")
				}
			}
		}()
		go func() {
			select {
			case <-a.ctx.Done():
			case <-exit.ClickedCh:
				a.forceQuit.Store(true)
				runtime.Quit(a.ctx)
			}
		}()
	}, func() {})
}

func stopTray() { systray.Quit() }
