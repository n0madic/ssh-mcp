package connection

import (
	"fmt"
	"io"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"
)

// bufferCompactThreshold is the read position beyond which the output buffer
// is compacted (copied down to index 0) to reclaim memory.
const bufferCompactThreshold = 1 << 20 // 1 MB

// TerminalID uniquely identifies a PTY terminal session.
type TerminalID string

// TerminalSession holds an active PTY session over SSH.
type TerminalSession struct {
	ID         TerminalID
	SessionID  SessionID
	sshSession *ssh.Session
	stdin      io.WriteCloser

	outputMu  sync.Mutex
	outputBuf []byte // accumulates all output since open
	readPos   int    // position up to which output has been returned

	outputNew chan struct{} // closed and recreated when new data arrives
	newMu     sync.Mutex

	// done is closed when all read goroutines have exited (via wg.Wait).
	// Blocked readers select on this to unblock immediately on close.
	done      chan struct{}
	wg        sync.WaitGroup
	closeOnce sync.Once

	mu        sync.Mutex
	closed    bool
	createdAt time.Time
	lastUsed  time.Time
}

// signalDone closes the done channel exactly once, waking any blocked readers.
func (ts *TerminalSession) signalDone() {
	ts.closeOnce.Do(func() { close(ts.done) })
}

// TerminalPool manages active PTY terminal sessions.
type TerminalPool struct {
	mu           sync.RWMutex
	sessions     map[TerminalID]*TerminalSession
	counter      atomic.Int64
	maxTerminals int
}

// NewTerminalPool creates a new TerminalPool.
// maxTerminals limits the number of concurrent sessions (0 = unlimited).
func NewTerminalPool(maxTerminals int) *TerminalPool {
	return &TerminalPool{
		sessions:     make(map[TerminalID]*TerminalSession),
		maxTerminals: maxTerminals,
	}
}

// Open allocates a PTY, starts an interactive shell, and returns a TerminalSession.
// cols and rows default to 120×50; termType defaults to "xterm-256color".
func (tp *TerminalPool) Open(sessionID SessionID, client *ssh.Client, cols, rows int, termType string) (*TerminalSession, error) {
	if cols <= 0 {
		cols = 120
	}
	if rows <= 0 {
		rows = 50
	}
	if termType == "" {
		termType = "xterm-256color"
	}

	sshSess, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("create SSH session: %w", err)
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := sshSess.RequestPty(termType, rows, cols, modes); err != nil {
		sshSess.Close()
		return nil, fmt.Errorf("request PTY: %w", err)
	}

	stdin, err := sshSess.StdinPipe()
	if err != nil {
		sshSess.Close()
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := sshSess.StdoutPipe()
	if err != nil {
		sshSess.Close()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	// Also capture stderr into the same buffer.
	stderr, err := sshSess.StderrPipe()
	if err != nil {
		sshSess.Close()
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := sshSess.Shell(); err != nil {
		sshSess.Close()
		return nil, fmt.Errorf("start shell: %w", err)
	}

	id := TerminalID(fmt.Sprintf("term-%d", tp.counter.Add(1)))
	now := time.Now()

	ts := &TerminalSession{
		ID:         id,
		SessionID:  sessionID,
		sshSession: sshSess,
		stdin:      stdin,
		outputNew:  make(chan struct{}),
		done:       make(chan struct{}),
		createdAt:  now,
		lastUsed:   now,
	}

	// Check pool limit with lock held before inserting.
	tp.mu.Lock()
	if tp.maxTerminals > 0 && len(tp.sessions) >= tp.maxTerminals {
		tp.mu.Unlock()
		stdin.Close()
		sshSess.Close()
		return nil, fmt.Errorf("maximum number of terminal sessions (%d) reached", tp.maxTerminals)
	}
	tp.sessions[id] = ts
	tp.mu.Unlock()

	// Start read goroutines with WaitGroup tracking.
	ts.wg.Add(2)
	go ts.readLoop(stdout)
	go ts.readLoop(stderr)

	// Signal done when both read goroutines complete.
	go func() {
		ts.wg.Wait()
		ts.signalDone()
	}()

	return ts, nil
}

// readLoop continuously reads from r and appends to outputBuf, signalling outputNew on each chunk.
func (ts *TerminalSession) readLoop(r io.Reader) {
	defer ts.wg.Done()

	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			ts.outputMu.Lock()
			ts.outputBuf = append(ts.outputBuf, buf[:n]...)
			ts.outputMu.Unlock()

			// Signal waiting readers by closing and replacing the channel.
			ts.newMu.Lock()
			ch := ts.outputNew
			ts.outputNew = make(chan struct{})
			close(ch)
			ts.newMu.Unlock()
		}
		if err != nil {
			if err != io.EOF {
				ts.mu.Lock()
				if !ts.closed {
					log.Printf("terminal %s read error: %v", ts.ID, err)
				}
				ts.mu.Unlock()
			}
			return
		}
	}
}

// Write sends data to the PTY stdin.
func (ts *TerminalSession) Write(data []byte) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.closed {
		return fmt.Errorf("terminal %s is closed", ts.ID)
	}
	ts.lastUsed = time.Now()
	_, err := ts.stdin.Write(data)
	return err
}

// terminalSettleTime is the quiet window used by ReadNewSince to decide that
// all output from a command has been received.
const terminalSettleTime = 100 * time.Millisecond

// BufLen returns the current number of bytes accumulated in the output buffer.
// Capture this before writing a command, then pass to ReadNewSince so it waits
// only for data produced *after* that point.
func (ts *TerminalSession) BufLen() int {
	ts.outputMu.Lock()
	defer ts.outputMu.Unlock()
	return len(ts.outputBuf)
}

// compactOutputBuf shifts unread data to the front of the buffer when readPos
// exceeds bufferCompactThreshold. Must be called under outputMu.
func (ts *TerminalSession) compactOutputBuf() {
	if ts.readPos < bufferCompactThreshold {
		return
	}
	remaining := len(ts.outputBuf) - ts.readPos
	if remaining > 0 {
		copy(ts.outputBuf, ts.outputBuf[ts.readPos:])
	}
	ts.outputBuf = ts.outputBuf[:remaining]
	ts.readPos = 0
}

// ReadNewSince waits until the output buffer grows past minLen, then enters a
// settling phase: it keeps resetting a short idle timer each time new data
// arrives, and returns once the terminal has been quiet for terminalSettleTime
// (or the overall deadline expires). This ensures multi-chunk responses
// (echo + output + prompt) are fully captured in a single call.
func (ts *TerminalSession) ReadNewSince(minLen int, waitDuration time.Duration) string {
	ts.mu.Lock()
	ts.lastUsed = time.Now()
	ts.mu.Unlock()

	if waitDuration > 0 {
		deadline := time.Now().Add(waitDuration)

		// Phase 1: wait for first new byte past minLen.
	phase1:
		for time.Now().Before(deadline) {
			ts.outputMu.Lock()
			bufLen := len(ts.outputBuf)
			ts.outputMu.Unlock()

			if bufLen > minLen {
				break
			}

			ts.newMu.Lock()
			ch := ts.outputNew
			ts.newMu.Unlock()

			remaining := time.Until(deadline)
			if remaining <= 0 {
				break
			}

			timer := time.NewTimer(remaining)
			select {
			case <-ch:
				timer.Stop()
			case <-ts.done:
				timer.Stop()
				break phase1
			case <-timer.C:
			}
		}

		// Phase 2: settling — keep extending the idle window while new data
		// keeps arriving, so trailing chunks (output + prompt) are all captured.
		settleDeadline := time.Now().Add(terminalSettleTime)
		if settleDeadline.After(deadline) {
			settleDeadline = deadline
		}
	phase2:
		for {
			ts.newMu.Lock()
			ch := ts.outputNew
			ts.newMu.Unlock()

			remaining := time.Until(settleDeadline)
			if remaining <= 0 {
				break
			}

			timer := time.NewTimer(remaining)
			select {
			case <-ch:
				timer.Stop()
				// More data arrived — push the settle deadline forward.
				next := time.Now().Add(terminalSettleTime)
				if next.After(deadline) {
					next = deadline
				}
				settleDeadline = next
			case <-ts.done:
				timer.Stop()
				break phase2
			case <-timer.C:
				// Quiet for terminalSettleTime — all output collected.
			}
		}
	}

	ts.outputMu.Lock()
	defer ts.outputMu.Unlock()

	if ts.readPos >= len(ts.outputBuf) {
		return ""
	}
	data := string(ts.outputBuf[ts.readPos:])
	ts.readPos = len(ts.outputBuf)
	ts.compactOutputBuf()
	return data
}

// ReadNew returns all output produced since the last call to ReadNew.
// If waitDuration > 0, it waits up to that duration for at least some new data.
func (ts *TerminalSession) ReadNew(waitDuration time.Duration) string {
	ts.mu.Lock()
	ts.lastUsed = time.Now()
	ts.mu.Unlock()

	// If caller wants to wait for data, poll the signal channel.
	if waitDuration > 0 {
		deadline := time.Now().Add(waitDuration)
	waitLoop:
		for time.Now().Before(deadline) {
			ts.outputMu.Lock()
			pos := ts.readPos
			bufLen := len(ts.outputBuf)
			ts.outputMu.Unlock()

			if bufLen > pos {
				break // new data available
			}

			ts.newMu.Lock()
			ch := ts.outputNew
			ts.newMu.Unlock()

			remaining := time.Until(deadline)
			if remaining <= 0 {
				break
			}

			timer := time.NewTimer(remaining)
			select {
			case <-ch:
				timer.Stop()
				// New data arrived, loop to check.
			case <-ts.done:
				timer.Stop()
				break waitLoop
			case <-timer.C:
				// Timeout reached.
			}
		}
	}

	ts.outputMu.Lock()
	defer ts.outputMu.Unlock()

	if ts.readPos >= len(ts.outputBuf) {
		return ""
	}
	data := string(ts.outputBuf[ts.readPos:])
	ts.readPos = len(ts.outputBuf)
	ts.compactOutputBuf()
	return data
}

// TerminalInfo provides read-only metadata about an active terminal session.
type TerminalInfo struct {
	ID        TerminalID
	SessionID SessionID
	CreatedAt time.Time
	LastUsed  time.Time
}

// List returns metadata for all active terminals. If sessionID is non-empty,
// only terminals belonging to that session are included.
func (tp *TerminalPool) List(sessionID SessionID) []TerminalInfo {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	result := make([]TerminalInfo, 0, len(tp.sessions))
	for _, ts := range tp.sessions {
		if sessionID != "" && ts.SessionID != sessionID {
			continue
		}
		ts.mu.Lock()
		info := TerminalInfo{
			ID:        ts.ID,
			SessionID: ts.SessionID,
			CreatedAt: ts.createdAt,
			LastUsed:  ts.lastUsed,
		}
		ts.mu.Unlock()
		result = append(result, info)
	}
	return result
}

// Get retrieves a TerminalSession by ID.
func (tp *TerminalPool) Get(id TerminalID) (*TerminalSession, error) {
	tp.mu.RLock()
	ts, ok := tp.sessions[id]
	tp.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("terminal %s not found", id)
	}
	return ts, nil
}

// closeSession closes a terminal session's resources and signals done.
func closeSession(ts *TerminalSession) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.closed = true
	if ts.stdin != nil {
		ts.stdin.Close()
	}
	if ts.sshSession != nil {
		ts.sshSession.Close()
	}
	ts.signalDone()
}

// Close terminates a TerminalSession and removes it from the pool.
func (tp *TerminalPool) Close(id TerminalID) error {
	tp.mu.Lock()
	ts, ok := tp.sessions[id]
	if ok {
		delete(tp.sessions, id)
	}
	tp.mu.Unlock()

	if !ok {
		return fmt.Errorf("terminal %s not found", id)
	}

	closeSession(ts)
	return nil
}

// CloseBySession closes all terminals associated with the given SSH session ID.
func (tp *TerminalPool) CloseBySession(sessionID SessionID) {
	tp.mu.Lock()
	var toClose []*TerminalSession
	for id, ts := range tp.sessions {
		if ts.SessionID == sessionID {
			toClose = append(toClose, ts)
			delete(tp.sessions, id)
		}
	}
	tp.mu.Unlock()

	for _, ts := range toClose {
		closeSession(ts)
	}
}

// InsertForTest adds a TerminalSession to the pool with default internal fields.
// Intended for use by external test packages that cannot access unexported fields.
func (tp *TerminalPool) InsertForTest(ts *TerminalSession) {
	now := time.Now()
	ts.done = make(chan struct{})
	ts.outputNew = make(chan struct{})
	ts.createdAt = now
	ts.lastUsed = now
	tp.mu.Lock()
	tp.sessions[ts.ID] = ts
	tp.mu.Unlock()
}

// CloseAll terminates all terminal sessions (used during server shutdown).
func (tp *TerminalPool) CloseAll() {
	tp.mu.Lock()
	sessions := make(map[TerminalID]*TerminalSession, len(tp.sessions))
	for id, ts := range tp.sessions {
		sessions[id] = ts
	}
	tp.sessions = make(map[TerminalID]*TerminalSession)
	tp.mu.Unlock()

	for _, ts := range sessions {
		closeSession(ts)
	}
}
