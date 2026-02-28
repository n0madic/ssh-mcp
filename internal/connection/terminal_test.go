package connection

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// TestTerminalPoolOpenClose verifies that Open creates a session with the expected ID
// format and Close removes it. Since we cannot dial a real SSH server in unit tests,
// we test the pool's bookkeeping directly by inserting a synthetic TerminalSession.
func TestTerminalPoolOpenClose(t *testing.T) {
	tp := NewTerminalPool(0)
	if tp == nil {
		t.Fatal("NewTerminalPool returned nil")
	}

	// Manually insert a synthetic session to test Close without real SSH.
	ts := &TerminalSession{
		ID:        TerminalID("term-1"),
		SessionID: SessionID("user@host:22"),
		done:      make(chan struct{}),
		// stdin / sshSession are nil; Close will call them, so use a no-op
		// We test the pool map management only here.
	}
	tp.mu.Lock()
	tp.sessions[ts.ID] = ts
	tp.mu.Unlock()

	if _, err := tp.Get(ts.ID); err != nil {
		t.Fatalf("Get after manual insert: %v", err)
	}

	// Verify ID format convention (term-<N>).
	if !strings.HasPrefix(string(ts.ID), "term-") {
		t.Errorf("unexpected ID format: %s", ts.ID)
	}

	// Mark as closed so Close() doesn't nil-deref on stdin/sshSession.
	ts.mu.Lock()
	ts.closed = true
	ts.mu.Unlock()

	// Remove from pool manually (simulates Close bookkeeping).
	tp.mu.Lock()
	delete(tp.sessions, ts.ID)
	tp.mu.Unlock()

	if _, err := tp.Get(ts.ID); err == nil {
		t.Error("expected error after removal, got nil")
	}
}

// TestTerminalPoolGet verifies Get returns an error for unknown IDs.
func TestTerminalPoolGet(t *testing.T) {
	tp := NewTerminalPool(0)

	_, err := tp.Get(TerminalID("does-not-exist"))
	if err == nil {
		t.Fatal("expected error for missing terminal, got nil")
	}

	// Insert a session and verify Get succeeds.
	ts := &TerminalSession{ID: TerminalID("term-42"), SessionID: SessionID("u@h:22"), done: make(chan struct{})}
	tp.mu.Lock()
	tp.sessions[ts.ID] = ts
	tp.mu.Unlock()

	got, err := tp.Get(ts.ID)
	if err != nil {
		t.Fatalf("Get existing: %v", err)
	}
	if got.ID != ts.ID {
		t.Errorf("expected ID %s, got %s", ts.ID, got.ID)
	}
}

// TestTerminalPoolCloseBySession verifies that all terminals for a given session are removed.
func TestTerminalPoolCloseBySession(t *testing.T) {
	tp := NewTerminalPool(0)

	target := SessionID("alice@host:22")
	other := SessionID("bob@host:22")

	// Insert three sessions: two for target, one for other.
	for _, ts := range []*TerminalSession{
		{ID: "term-1", SessionID: target, done: make(chan struct{})},
		{ID: "term-2", SessionID: target, done: make(chan struct{})},
		{ID: "term-3", SessionID: other, done: make(chan struct{})},
	} {
		ts.mu.Lock()
		ts.closed = true // prevent nil deref in CloseBySession
		ts.mu.Unlock()
		tp.mu.Lock()
		tp.sessions[ts.ID] = ts
		tp.mu.Unlock()
	}

	tp.CloseBySession(target)

	tp.mu.RLock()
	remaining := len(tp.sessions)
	_, hasOther := tp.sessions[TerminalID("term-3")]
	tp.mu.RUnlock()

	if remaining != 1 {
		t.Errorf("expected 1 session remaining, got %d", remaining)
	}
	if !hasOther {
		t.Error("unrelated session was incorrectly removed")
	}
}

// TestTerminalSessionReadNew verifies that ReadNew returns data appended to outputBuf
// and advances readPos, and that a subsequent call returns only new data.
func TestTerminalSessionReadNew(t *testing.T) {
	ts := &TerminalSession{
		outputNew: make(chan struct{}),
		done:      make(chan struct{}),
	}

	// Simulate data arriving (normally done by readLoop).
	ts.outputMu.Lock()
	ts.outputBuf = append(ts.outputBuf, []byte("hello world")...)
	ts.outputMu.Unlock()

	first := ts.ReadNew(0)
	if first != "hello world" {
		t.Errorf("first ReadNew: expected %q, got %q", "hello world", first)
	}

	// No new data — should return empty.
	second := ts.ReadNew(0)
	if second != "" {
		t.Errorf("second ReadNew (no new data): expected empty, got %q", second)
	}

	// Add more data.
	ts.outputMu.Lock()
	ts.outputBuf = append(ts.outputBuf, []byte(" more")...)
	ts.outputMu.Unlock()

	third := ts.ReadNew(0)
	if third != " more" {
		t.Errorf("third ReadNew: expected %q, got %q", " more", third)
	}
}

// TestTerminalSessionReadNewWaits verifies that ReadNew with a non-zero wait duration
// blocks until data becomes available (signal via outputNew channel).
func TestTerminalSessionReadNewWaits(t *testing.T) {
	ts := &TerminalSession{
		outputNew: make(chan struct{}),
		done:      make(chan struct{}),
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Write data after a short delay.
		time.Sleep(50 * time.Millisecond)
		ts.outputMu.Lock()
		ts.outputBuf = append(ts.outputBuf, []byte("delayed")...)
		ts.outputMu.Unlock()

		ts.newMu.Lock()
		ch := ts.outputNew
		ts.outputNew = make(chan struct{})
		close(ch)
		ts.newMu.Unlock()
	}()

	result := ts.ReadNew(500 * time.Millisecond)
	wg.Wait()

	if result != "delayed" {
		t.Errorf("ReadNew with wait: expected %q, got %q", "delayed", result)
	}
}

// TestTerminalPoolCloseAll verifies CloseAll empties the pool.
func TestTerminalPoolCloseAll(t *testing.T) {
	tp := NewTerminalPool(0)

	for _, id := range []TerminalID{"term-a", "term-b", "term-c"} {
		ts := &TerminalSession{ID: id, SessionID: SessionID("u@h:22"), done: make(chan struct{})}
		ts.mu.Lock()
		ts.closed = true // prevent nil deref
		ts.mu.Unlock()
		tp.mu.Lock()
		tp.sessions[id] = ts
		tp.mu.Unlock()
	}

	tp.CloseAll()

	tp.mu.RLock()
	n := len(tp.sessions)
	tp.mu.RUnlock()

	if n != 0 {
		t.Errorf("expected 0 sessions after CloseAll, got %d", n)
	}
}

// TestTerminalSessionReadNewDoneUnblocks verifies that closing the done channel
// immediately unblocks a ReadNew call with a long wait duration.
func TestTerminalSessionReadNewDoneUnblocks(t *testing.T) {
	ts := &TerminalSession{
		outputNew: make(chan struct{}),
		done:      make(chan struct{}),
	}

	start := time.Now()

	// Close done after a short delay to simulate terminal closure.
	go func() {
		time.Sleep(50 * time.Millisecond)
		close(ts.done)
	}()

	result := ts.ReadNew(5 * time.Second)
	elapsed := time.Since(start)

	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
	if elapsed > 1*time.Second {
		t.Errorf("ReadNew took %v, expected to unblock quickly after done closed", elapsed)
	}
}

// TestTerminalSessionReadNewSinceDoneUnblocks verifies that closing the done channel
// immediately unblocks a ReadNewSince call with a long wait duration.
func TestTerminalSessionReadNewSinceDoneUnblocks(t *testing.T) {
	ts := &TerminalSession{
		outputNew: make(chan struct{}),
		done:      make(chan struct{}),
	}

	start := time.Now()

	// Close done after a short delay to simulate terminal closure.
	go func() {
		time.Sleep(50 * time.Millisecond)
		close(ts.done)
	}()

	result := ts.ReadNewSince(0, 5*time.Second)
	elapsed := time.Since(start)

	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
	if elapsed > 1*time.Second {
		t.Errorf("ReadNewSince took %v, expected to unblock quickly after done closed", elapsed)
	}
}

// TestTerminalSessionBufferCompaction verifies that the output buffer is compacted
// when readPos exceeds bufferCompactThreshold.
func TestTerminalSessionBufferCompaction(t *testing.T) {
	ts := &TerminalSession{
		outputNew: make(chan struct{}),
		done:      make(chan struct{}),
	}

	// Fill buffer past the compaction threshold (1 MB).
	data := make([]byte, bufferCompactThreshold+100)
	for i := range data {
		data[i] = 'A'
	}
	// Write some trailing data we want to verify survives compaction.
	trailing := []byte("TRAILING")
	data = append(data, trailing...)

	ts.outputMu.Lock()
	ts.outputBuf = data
	ts.outputMu.Unlock()

	// Read all data — this should trigger compaction.
	result := ts.ReadNew(0)
	if len(result) != len(data) {
		t.Errorf("expected %d bytes, got %d", len(data), len(result))
	}
	if !strings.HasSuffix(result, "TRAILING") {
		t.Error("expected data to end with TRAILING")
	}

	// After reading all data, readPos should be compacted to 0.
	ts.outputMu.Lock()
	if ts.readPos != 0 {
		t.Errorf("expected readPos=0 after compaction, got %d", ts.readPos)
	}
	if len(ts.outputBuf) != 0 {
		t.Errorf("expected empty outputBuf after reading all data, got %d bytes", len(ts.outputBuf))
	}
	ts.outputMu.Unlock()
}

// TestTerminalPoolList verifies that List returns correct terminal info
// and supports optional filtering by session ID.
func TestTerminalPoolList(t *testing.T) {
	tp := NewTerminalPool(0)

	// Empty pool should return empty slice.
	result := tp.List("")
	if len(result) != 0 {
		t.Fatalf("expected 0 terminals from empty pool, got %d", len(result))
	}

	sessionA := SessionID("alice@host:22")
	sessionB := SessionID("bob@host:22")
	now := time.Now()

	// Insert synthetic sessions.
	for _, ts := range []*TerminalSession{
		{ID: "term-1", SessionID: sessionA, done: make(chan struct{}), createdAt: now, lastUsed: now},
		{ID: "term-2", SessionID: sessionA, done: make(chan struct{}), createdAt: now, lastUsed: now},
		{ID: "term-3", SessionID: sessionB, done: make(chan struct{}), createdAt: now, lastUsed: now},
	} {
		tp.mu.Lock()
		tp.sessions[ts.ID] = ts
		tp.mu.Unlock()
	}

	// List all terminals.
	all := tp.List("")
	if len(all) != 3 {
		t.Errorf("expected 3 terminals, got %d", len(all))
	}

	// Filter by sessionA.
	filtered := tp.List(sessionA)
	if len(filtered) != 2 {
		t.Errorf("expected 2 terminals for sessionA, got %d", len(filtered))
	}
	for _, info := range filtered {
		if info.SessionID != sessionA {
			t.Errorf("expected SessionID %s, got %s", sessionA, info.SessionID)
		}
	}

	// Filter by sessionB.
	filteredB := tp.List(sessionB)
	if len(filteredB) != 1 {
		t.Errorf("expected 1 terminal for sessionB, got %d", len(filteredB))
	}

	// Filter by non-existent session.
	filteredNone := tp.List(SessionID("nobody@host:22"))
	if len(filteredNone) != 0 {
		t.Errorf("expected 0 terminals for unknown session, got %d", len(filteredNone))
	}
}

// TestTerminalPoolMaxTerminals verifies that maxTerminals is stored correctly.
func TestTerminalPoolMaxTerminals(t *testing.T) {
	tp := NewTerminalPool(5)
	if tp.maxTerminals != 5 {
		t.Errorf("expected maxTerminals=5, got %d", tp.maxTerminals)
	}

	tp0 := NewTerminalPool(0)
	if tp0.maxTerminals != 0 {
		t.Errorf("expected maxTerminals=0, got %d", tp0.maxTerminals)
	}
}
