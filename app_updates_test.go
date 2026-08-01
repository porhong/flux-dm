package main

import (
	"strings"
	"testing"

	"github.com/fluxdm/fluxdm/internal/application"
)

func TestPortableBuildReportsManualUpdatePath(t *testing.T) {
	previous := application.PortableMode
	application.PortableMode = "true"
	t.Cleanup(func() { application.PortableMode = previous })

	_, err := NewApp(application.Paths{}, nil).GetUpdateStatus()
	if err == nil {
		t.Fatal("expected portable update status to be unavailable")
	}
	if !strings.Contains(err.Error(), "replacing the extracted FluxDM folder") {
		t.Fatalf("unexpected portable update message: %q", err)
	}
}
