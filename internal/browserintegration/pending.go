package browserintegration

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"
)

// DefaultPendingTTL is how long a browser-originated download request is
// remembered before the user must retry from the browser. Cookies attached
// to a pending request are removed from memory when the entry is taken,
// discarded, or expires.
const DefaultPendingTTL = 5 * time.Minute

// PendingRequest holds the still-unconfirmed payload from a single browser
// download handoff. The Cookies field never leaves this package: it is
// consumed by the confirmation path and never included in events emitted to
// the Wails frontend.
type PendingRequest struct {
	ID                string
	URL               string
	SuggestedFilename string
	Referrer          string
	Cookies           string
	CreatedAt         time.Time
	ExpiresAt         time.Time
}

// PendingStore is a bounded in-memory cache of unconfirmed browser download
// requests keyed by an opaque, unguessable identifier. Entries expire
// lazily on access; a request that is never confirmed or discarded is
// collected the next time any other entry is touched.
type PendingStore struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]pendingEntry
}

type pendingEntry struct {
	request PendingRequest
	claimed bool
}

// NewPendingStore returns a store whose entries expire after ttl. Pass zero
// to use DefaultPendingTTL.
func NewPendingStore(ttl time.Duration) *PendingStore {
	if ttl <= 0 {
		ttl = DefaultPendingTTL
	}
	return &PendingStore{ttl: ttl, entries: make(map[string]pendingEntry)}
}

// Put stores a copy of the request and returns the opaque identifier the
// frontend uses to confirm or discard it. It also opportunistically
// removes expired entries so abandoned requests cannot accumulate.
func (s *PendingStore) Put(now time.Time, request PendingRequest) string {
	id := newPendingID()
	request.ID = id
	request.CreatedAt = now
	request.ExpiresAt = now.Add(s.ttl)
	s.mu.Lock()
	s.sweepLocked(now)
	s.entries[id] = pendingEntry{request: request}
	s.mu.Unlock()
	return id
}

// Take removes and returns the request for id. Returns ok=false if the
// identifier is unknown or the entry has expired.
func (s *PendingStore) Take(now time.Time, id string) (PendingRequest, bool) {
	if id == "" {
		return PendingRequest{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[id]
	if !ok {
		return PendingRequest{}, false
	}
	delete(s.entries, id)
	if now.After(entry.request.ExpiresAt) {
		return PendingRequest{}, false
	}
	return entry.request, true
}

// Claim returns a request exclusively while leaving it available for retry if
// confirmation fails. The caller must Release the claim on failure or
// Complete it after successfully creating the download record.
func (s *PendingStore) Claim(now time.Time, id string) (PendingRequest, bool) {
	if id == "" {
		return PendingRequest{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked(now)
	entry, ok := s.entries[id]
	if !ok || entry.claimed {
		return PendingRequest{}, false
	}
	entry.claimed = true
	s.entries[id] = entry
	return entry.request, true
}

// Release makes a claimed request available for another confirmation attempt.
// It is used when validation or record creation fails, so the user can choose
// a different destination without having to repeat the browser handoff.
func (s *PendingStore) Release(now time.Time, id string) {
	if id == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked(now)
	entry, ok := s.entries[id]
	if !ok {
		return
	}
	entry.claimed = false
	s.entries[id] = entry
}

// Complete removes a successfully confirmed request and its cookies.
func (s *PendingStore) Complete(id string) {
	if id == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, id)
}

// Discard removes a pending request without returning it. Expired entries
// are also collected.
func (s *PendingStore) Discard(now time.Time, id string) {
	if id == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, id)
	s.sweepLocked(now)
}

// List returns copies of every non-expired pending request. Callers must
// avoid exposing Cookies outside the trusted backend process.
func (s *PendingStore) List(now time.Time) []PendingRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked(now)
	requests := make([]PendingRequest, 0, len(s.entries))
	for _, entry := range s.entries {
		if entry.claimed {
			continue
		}
		requests = append(requests, entry.request)
	}
	return requests
}

// Len reports the current number of stored entries, including any that have
// expired but have not yet been swept.
func (s *PendingStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

// sweepLocked removes expired entries. Caller must hold s.mu.
func (s *PendingStore) sweepLocked(now time.Time) {
	for id, entry := range s.entries {
		if now.After(entry.request.ExpiresAt) {
			delete(s.entries, id)
		}
	}
}

// newPendingID returns a 32-character hex identifier from 128 random bits.
// The identifier is opaque to callers and unguessable, which is important
// because it is the only handle tying a Wails confirmation call back to a
// specific browser request that carried session cookies.
func newPendingID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return fmt.Sprintf("%016x", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", value[:])
}
