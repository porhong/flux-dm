package browserintegration

import (
	"sync"
	"testing"
	"time"
)

func TestPendingStorePutTakeRoundTrip(t *testing.T) {
	store := NewPendingStore(time.Minute)
	now := time.Now()
	id := store.Put(now, PendingRequest{
		URL:     "https://example.test/file.zip",
		Cookies: "session=abc",
	})
	if id == "" {
		t.Fatal("empty pending id")
	}
	if store.Len() != 1 {
		t.Fatalf("len=%d", store.Len())
	}
	got, ok := store.Take(now, id)
	if !ok {
		t.Fatal("missing entry")
	}
	if got.URL != "https://example.test/file.zip" || got.Cookies != "session=abc" {
		t.Fatalf("payload mismatch: %#v", got)
	}
	if store.Len() != 0 {
		t.Fatalf("entry not removed, len=%d", store.Len())
	}
	if _, ok := store.Take(now, id); ok {
		t.Fatal("take returned the same id twice")
	}
}

func TestPendingStoreTakeRejectsUnknownAndExpired(t *testing.T) {
	store := NewPendingStore(50 * time.Millisecond)
	start := time.Now()
	id := store.Put(start, PendingRequest{URL: "https://example.test/a"})

	if _, ok := store.Take(start, "unknown"); ok {
		t.Fatal("unknown id should not be taken")
	}
	if _, ok := store.Take(start, ""); ok {
		t.Fatal("empty id should not be taken")
	}
	if _, ok := store.Take(start.Add(100*time.Millisecond), id); ok {
		t.Fatal("expired entry should not be takeable")
	}
	if store.Len() != 0 {
		t.Fatalf("expired entry was not removed, len=%d", store.Len())
	}
}

func TestPendingStoreClaimReleaseAndComplete(t *testing.T) {
	store := NewPendingStore(time.Minute)
	now := time.Now()
	id := store.Put(now, PendingRequest{URL: "https://example.test/file.zip", Cookies: "session=abc"})

	claimed, ok := store.Claim(now, id)
	if !ok || claimed.Cookies != "session=abc" {
		t.Fatalf("claim=%+v ok=%t", claimed, ok)
	}
	if _, ok := store.Claim(now, id); ok {
		t.Fatal("claimed entry should not be claimable twice")
	}
	if listed := store.List(now); len(listed) != 0 {
		t.Fatalf("claimed entry should not be listed, got %d", len(listed))
	}

	store.Release(now, id)
	if _, ok := store.Claim(now, id); !ok {
		t.Fatal("released entry should be claimable again")
	}
	store.Complete(id)
	if store.Len() != 0 {
		t.Fatalf("completed entry not removed, len=%d", store.Len())
	}
}

func TestPendingStoreDiscardRemovesEntry(t *testing.T) {
	store := NewPendingStore(time.Minute)
	now := time.Now()
	id := store.Put(now, PendingRequest{URL: "https://example.test/x"})
	store.Discard(now, id)
	if store.Len() != 0 {
		t.Fatalf("discard did not remove entry, len=%d", store.Len())
	}
	if _, ok := store.Take(now, id); ok {
		t.Fatal("discarded entry was taken")
	}
}

func TestPendingStoreListExcludesExpiredEntries(t *testing.T) {
	store := NewPendingStore(10 * time.Millisecond)
	start := time.Now()
	store.Put(start, PendingRequest{URL: "https://example.test/expired"})
	activeID := store.Put(start.Add(20*time.Millisecond), PendingRequest{URL: "https://example.test/active"})

	requests := store.List(start.Add(20 * time.Millisecond))
	if len(requests) != 1 {
		t.Fatalf("list length=%d want=1", len(requests))
	}
	if requests[0].URL != "https://example.test/active" {
		t.Fatalf("list URL=%q", requests[0].URL)
	}
	if _, ok := store.Take(start.Add(20*time.Millisecond), activeID); !ok {
		t.Fatal("listed active request should remain available")
	}
}

func TestPendingStorePutSweepsExpiredEntries(t *testing.T) {
	store := NewPendingStore(10 * time.Millisecond)
	t0 := time.Now()
	store.Put(t0, PendingRequest{URL: "https://example.test/old1"})
	store.Put(t0, PendingRequest{URL: "https://example.test/old2"})
	if store.Len() != 2 {
		t.Fatalf("len=%d after puts", store.Len())
	}
	store.Put(t0.Add(50*time.Millisecond), PendingRequest{URL: "https://example.test/new"})
	if store.Len() != 1 {
		t.Fatalf("expired entries not swept, len=%d", store.Len())
	}
}

func TestPendingStoreConcurrentAccess(t *testing.T) {
	store := NewPendingStore(time.Minute)
	now := time.Now()
	const workers = 16
	const iterations = 64
	var wg sync.WaitGroup
	wg.Add(workers * 2)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				store.Put(now, PendingRequest{URL: "https://example.test/c"})
			}
		}()
	}
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, _ = store.Take(now, "missing")
			}
		}()
	}
	wg.Wait()
	if store.Len() != workers*iterations {
		t.Fatalf("len=%d want=%d", store.Len(), workers*iterations)
	}
}
