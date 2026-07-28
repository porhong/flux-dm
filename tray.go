package main

import (
	"context"
	_ "embed"
	goruntime "runtime"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed build/windows/icon.ico
var trayIcon []byte

func (a *App) startTray() {
	a.trayMu.Lock()
	if a.trayStarted {
		a.trayMu.Unlock()
		return
	}
	stop := make(chan struct{})
	ready := make(chan struct{})
	done := make(chan struct{})
	a.trayStarted = true
	a.trayStop = stop
	a.trayReady = ready
	a.trayDone = done
	ctx := a.ctx
	a.trayMu.Unlock()

	go a.runTray(ctx, stop, ready, done)
}

// runTray owns the native tray window and its message loop. Windows requires
// both to remain on the same OS thread; moving the loop between threads can
// leave an icon visible while its click callbacks no longer display a menu.
func (a *App) runTray(ctx context.Context, stop <-chan struct{}, ready chan<- struct{}, done chan<- struct{}) {
	goruntime.LockOSThread()
	defer goruntime.UnlockOSThread()
	defer close(done)

	systray.Run(func() {
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
				case <-ctx.Done():
					return
				case <-stop:
					return
				case <-show.ClickedCh:
					runtime.WindowShow(ctx)
					runtime.WindowUnminimise(ctx)
				}
			}
		}()
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-stop:
					return
				case <-add.ClickedCh:
					runtime.WindowShow(ctx)
					runtime.WindowUnminimise(ctx)
					runtime.EventsEmit(ctx, "tray:add-download")
				}
			}
		}()
		go func() {
			select {
			case <-ctx.Done():
			case <-stop:
			case <-exit.ClickedCh:
				a.forceQuit.Store(true)
				runtime.Quit(ctx)
			}
		}()
		close(ready)
	}, func() {})
}

func (a *App) stopTray(ctx context.Context) {
	a.trayMu.Lock()
	if !a.trayStarted {
		a.trayMu.Unlock()
		return
	}
	stop := a.trayStop
	ready := a.trayReady
	done := a.trayDone
	a.trayStarted = false
	a.trayStop = nil
	a.trayReady = nil
	a.trayDone = nil
	a.trayMu.Unlock()

	close(stop)
	select {
	case <-ready:
		systray.Quit()
	case <-done:
		return
	case <-ctx.Done():
		return
	}
	select {
	case <-done:
	case <-ctx.Done():
	}
}
