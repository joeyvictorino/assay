//go:build zdrdebug

package zdr

import (
	"context"
	"sync"
)

// MemorySink keeps transcripts in memory. It exists only in binaries built
// with -tags zdrdebug, so a release build cannot retain a transcript even by
// misconfiguration.
type MemorySink struct {
	mu      sync.Mutex
	entries []MemoryEntry
}

// MemoryEntry is one retained transcript.
type MemoryEntry struct {
	RunID string
	Agent string
	Body  []byte
}

// Write implements model.TranscriptSink by copying body into memory.
func (m *MemorySink) Write(_ context.Context, runID, agent string, body []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, MemoryEntry{RunID: runID, Agent: agent, Body: append([]byte(nil), body...)})
	return nil
}

// Entries returns a copy of everything written so far.
func (m *MemorySink) Entries() []MemoryEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]MemoryEntry(nil), m.entries...)
}
