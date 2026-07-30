package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/fluxdm/fluxdm/internal/update"
)

func main() {
	parentPID := flag.Int("parent-pid", 0, "FluxDM process to wait for")
	installer := flag.String("installer", "", "verified installer path")
	restart := flag.String("restart", "", "installed FluxDM executable")
	targetVersion := flag.String("target-version", "", "target FluxDM release version")
	token := flag.String("token", "", "private installation handoff token")
	resultPath := flag.String("result", "", "handoff result path")
	flag.Parse()
	if *parentPID < 1 || !validEXE(*installer) || !validEXE(*restart) || !filepath.IsAbs(*resultPath) {
		fmt.Fprintln(os.Stderr, "invalid update launcher arguments")
		os.Exit(2)
	}
	handoff := update.Handoff{TargetVersion: *targetVersion, Token: *token, ResultPath: *resultPath}
	writeResult := func(succeeded bool, failure string) {
		_ = update.WriteHandoffResult(handoff.ResultPath, update.HandoffResult{TargetVersion: handoff.TargetVersion, Token: handoff.Token, Succeeded: succeeded, Failure: failure})
	}
	parent, err := os.FindProcess(*parentPID)
	if err != nil {
		writeResult(false, "parent_exit_timeout")
		fmt.Fprintln(os.Stderr, "could not wait for FluxDM")
		os.Exit(1)
	}
	done := make(chan error, 1)
	go func() { _, waitErr := parent.Wait(); done <- waitErr }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		writeResult(false, "parent_exit_timeout")
		fmt.Fprintln(os.Stderr, "FluxDM did not exit in time")
		os.Exit(1)
	}
	command := exec.Command(*installer, "/S")
	if err := command.Run(); err != nil {
		writeResult(false, "installer_failed")
		fmt.Fprintln(os.Stderr, "update installer failed")
		os.Exit(1)
	}
	if err := exec.Command(*restart).Start(); err != nil {
		writeResult(false, "restart_failed")
		fmt.Fprintln(os.Stderr, "could not restart FluxDM")
		os.Exit(1)
	}
	writeResult(true, "")
}

func validEXE(path string) bool {
	absolute, err := filepath.Abs(path)
	return err == nil && absolute == path && filepath.Ext(path) == ".exe" && path != "" && strconv.Quote(path) != ""
}
