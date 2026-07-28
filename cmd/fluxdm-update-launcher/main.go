package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

func main() {
	parentPID := flag.Int("parent-pid", 0, "FluxDM process to wait for")
	installer := flag.String("installer", "", "verified installer path")
	restart := flag.String("restart", "", "installed FluxDM executable")
	flag.Parse()
	if *parentPID < 1 || !validEXE(*installer) || !validEXE(*restart) {
		fmt.Fprintln(os.Stderr, "invalid update launcher arguments")
		os.Exit(2)
	}
	parent, err := os.FindProcess(*parentPID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	done := make(chan error, 1)
	go func() { _, waitErr := parent.Wait(); done <- waitErr }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		fmt.Fprintln(os.Stderr, "FluxDM did not exit in time")
		os.Exit(1)
	}
	command := exec.Command(*installer)
	if err := command.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := exec.Command(*restart).Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func validEXE(path string) bool {
	absolute, err := filepath.Abs(path)
	return err == nil && absolute == path && filepath.Ext(path) == ".exe" && path != "" && strconv.Quote(path) != ""
}
