package interp

import (
	"context"
	"strings"
	"sync"
	"time"
)

// DuplicateTracker backs the duplicate test (RFC 7352).
// IsDuplicate atomically checks and records the tracking ID scoped by handle.
// Returns true if the ID was already present (duplicate), false if new.
// seconds is the TTL for new entries; when last=true the TTL is refreshed on
// existing entries too (":last" tag semantics).
// If DuplicateTracker is nil on RuntimeData the duplicate test always returns false.
type DuplicateTracker interface {
	IsDuplicate(ctx context.Context, handle, id string, seconds uint32, last bool) (bool, error)
}

// MemoryDuplicateTracker is a simple in-memory DuplicateTracker.
// Suitable for testing and single-process deployments.
type MemoryDuplicateTracker struct {
	mu      sync.Mutex
	entries map[string]time.Time // key → expiry
}

func NewMemoryDuplicateTracker() *MemoryDuplicateTracker {
	return &MemoryDuplicateTracker{entries: map[string]time.Time{}}
}

func (t *MemoryDuplicateTracker) IsDuplicate(_ context.Context, handle, id string, seconds uint32, last bool) (bool, error) {
	key := handle + "\x00" + id
	now := time.Now()
	expiry := now.Add(time.Duration(seconds) * time.Second)

	t.mu.Lock()
	defer t.mu.Unlock()

	t.evict(now)

	if prev, ok := t.entries[key]; ok && prev.After(now) {
		if last {
			t.entries[key] = expiry
		}
		return true, nil
	}
	t.entries[key] = expiry
	return false, nil
}

func (t *MemoryDuplicateTracker) evict(now time.Time) {
	for k, exp := range t.entries {
		if !exp.After(now) {
			delete(t.entries, k)
		}
	}
}

const duplicateDefaultSeconds uint32 = 7 * 24 * 3600 // 7 days

type idSourceKind int

const (
	idSourceMessageID idSourceKind = iota
	idSourceHeader
	idSourceUniqueID
)

type TestDuplicate struct {
	Handle  string
	Seconds uint32
	Last    bool

	idKind  idSourceKind
	idParam string // header name or explicit uniqueid value
}

func (t *TestDuplicate) Check(ctx context.Context, d *RuntimeData) (bool, error) {
	if d.DuplicateTracker == nil {
		return false, nil
	}
	if t.Seconds == 0 {
		return false, nil
	}

	id, ok := t.resolveID(d)
	if !ok || id == "" {
		return false, nil
	}

	return d.DuplicateTracker.IsDuplicate(ctx, t.Handle, id, t.Seconds, t.Last)
}

func (t *TestDuplicate) resolveID(d *RuntimeData) (string, bool) {
	switch t.idKind {
	case idSourceUniqueID:
		return expandVars(d, t.idParam), true
	case idSourceHeader:
		name := t.idParam
		vals, err := d.Msg.HeaderGet(name)
		if err != nil || len(vals) == 0 {
			return "", false
		}
		return strings.TrimSpace(vals[0]), true
	default: // idSourceMessageID
		vals, err := d.Msg.HeaderGet("Message-ID")
		if err != nil || len(vals) == 0 {
			return "", false
		}
		return strings.TrimSpace(vals[0]), true
	}
}
