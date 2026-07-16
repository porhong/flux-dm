package application

import (
	"context"
	"testing"
)

func TestNextJobHonorsPriorityAndQueueCapacity(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service := &DownloadService{
		ctx: ctx, cancel: cancel, wake: make(chan struct{}, 1),
		pending: []downloadJob{
			{id: "low", queueID: "a", priority: 1, maxParallel: 2},
			{id: "blocked-high", queueID: "b", priority: 100, maxParallel: 1},
			{id: "high", queueID: "a", priority: 5, maxParallel: 2},
		},
		queued:       map[string]struct{}{"low": {}, "blocked-high": {}, "high": {}},
		queueRunning: map[string]int{"b": 1},
		running:      make(map[string]*runControl),
	}
	job, ok := service.nextJob()
	if !ok || job.id != "high" {
		t.Fatalf("expected highest eligible job, got %#v", job)
	}
	if service.queueRunning["a"] != 1 {
		t.Fatalf("expected queue slot reservation")
	}
}
