package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/n0madic/ssh-mcp/internal/connection"
	"github.com/n0madic/ssh-mcp/internal/security"
)

// escapeReplacer expands common escape sequences in terminal text input.
var escapeReplacer = strings.NewReplacer(`\n`, "\n", `\r`, "\r", `\t`, "\t")

// specialKeys maps human-readable key names to their byte sequences.
var specialKeys = map[string][]byte{
	"CTRL_C":      {0x03},
	"CTRL_D":      {0x04},
	"CTRL_Z":      {0x1a},
	"ESC":         {0x1b},
	"TAB":         {0x09},
	"BACKSPACE":   {0x7f},
	"ENTER":       {'\r'},
	"ARROW_UP":    {0x1b, '[', 'A'},
	"ARROW_DOWN":  {0x1b, '[', 'B'},
	"ARROW_RIGHT": {0x1b, '[', 'C'},
	"ARROW_LEFT":  {0x1b, '[', 'D'},
}

// TerminalDeps holds dependencies for terminal tool handlers.
type TerminalDeps struct {
	Pool          *connection.Pool
	TermPool      *connection.TerminalPool
	RateLimiter   *security.RateLimiter
	MaxOutputSize int
}

// HandleOpenTerminal opens a new interactive PTY terminal session.
func HandleOpenTerminal(ctx context.Context, deps *TerminalDeps, input SSHOpenTerminalInput) (*SSHOpenTerminalOutput, error) {
	if input.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	conn, err := deps.Pool.GetConnection(ctx, connection.SessionID(input.SessionID))
	if err != nil {
		return nil, err
	}

	// Rate limit terminal open operations.
	if deps.RateLimiter != nil {
		if err := deps.RateLimiter.Allow(conn.Host); err != nil {
			return nil, err
		}
	}

	// Apply defaults before calling Open so the message reflects actual values.
	cols := input.Cols
	if cols <= 0 {
		cols = 120
	}
	rows := input.Rows
	if rows <= 0 {
		rows = 50
	}

	waitMs := input.WaitMs
	if waitMs <= 0 {
		waitMs = 500
	}

	ts, err := deps.TermPool.Open(connection.SessionID(input.SessionID), conn.Client, cols, rows, input.TermType)
	if err != nil {
		return nil, fmt.Errorf("open terminal: %w", err)
	}

	// Wait for initial shell prompt.
	output := TruncateOutput(ts.ReadNew(time.Duration(waitMs)*time.Millisecond), deps.MaxOutputSize)

	return &SSHOpenTerminalOutput{
		TerminalID: string(ts.ID),
		Output:     output,
		Message:    fmt.Sprintf("PTY terminal opened (cols=%d, rows=%d)", cols, rows),
	}, nil
}

// HandleSendInput writes text or a special key to a terminal's stdin and reads back new output.
func HandleSendInput(ctx context.Context, deps *TerminalDeps, input SSHSendInputInput) (*SSHSendInputOutput, error) {
	if input.TerminalID == "" {
		return nil, fmt.Errorf("terminal_id is required")
	}
	if input.Text != "" && input.SpecialKey != "" {
		return nil, fmt.Errorf("only one of text or special_key can be provided, not both")
	}
	if input.Text == "" && input.SpecialKey == "" {
		return nil, fmt.Errorf("either text or special_key must be provided")
	}

	ts, err := deps.TermPool.Get(connection.TerminalID(input.TerminalID))
	if err != nil {
		return nil, err
	}

	// Rate limit terminal write operations.
	if deps.RateLimiter != nil {
		conn, connErr := deps.Pool.GetConnection(ctx, ts.SessionID)
		if connErr == nil {
			if err := deps.RateLimiter.Allow(conn.Host); err != nil {
				return nil, err
			}
		}
	}

	var payload []byte
	if input.SpecialKey != "" {
		seq, ok := specialKeys[input.SpecialKey]
		if !ok {
			return nil, fmt.Errorf("unknown special key %q; valid keys: CTRL_C, CTRL_D, CTRL_Z, ESC, TAB, BACKSPACE, ENTER, ARROW_UP, ARROW_DOWN, ARROW_LEFT, ARROW_RIGHT", input.SpecialKey)
		}
		payload = seq
	} else {
		payload = []byte(escapeReplacer.Replace(input.Text))
	}

	// Snapshot buffer length before writing so ReadNewSince waits only for
	// output produced *after* this command, ignoring any stale buffered data.
	startLen := ts.BufLen()

	if err := ts.Write(payload); err != nil {
		return nil, fmt.Errorf("write to terminal: %w", err)
	}

	waitMs := input.WaitMs
	if waitMs <= 0 {
		waitMs = 300
	}

	output := TruncateOutput(ts.ReadNewSince(startLen, time.Duration(waitMs)*time.Millisecond), deps.MaxOutputSize)

	return &SSHSendInputOutput{
		Output:  output,
		Written: len(payload),
	}, nil
}

// HandleReadOutput reads buffered output from a terminal since the last read.
func HandleReadOutput(ctx context.Context, deps *TerminalDeps, input SSHReadOutputInput) (*SSHReadOutputOutput, error) {
	if input.TerminalID == "" {
		return nil, fmt.Errorf("terminal_id is required")
	}

	ts, err := deps.TermPool.Get(connection.TerminalID(input.TerminalID))
	if err != nil {
		return nil, err
	}

	wait := time.Duration(input.WaitMs) * time.Millisecond
	output := TruncateOutput(ts.ReadNew(wait), deps.MaxOutputSize)

	return &SSHReadOutputOutput{
		Output: output,
		HasNew: output != "",
	}, nil
}

// HandleCloseTerminal closes an active PTY terminal session.
func HandleCloseTerminal(ctx context.Context, deps *TerminalDeps, input SSHCloseTerminalInput) (*SSHCloseTerminalOutput, error) {
	if input.TerminalID == "" {
		return nil, fmt.Errorf("terminal_id is required")
	}

	if err := deps.TermPool.Close(connection.TerminalID(input.TerminalID)); err != nil {
		return nil, err
	}

	return &SSHCloseTerminalOutput{
		Message: fmt.Sprintf("Terminal %s closed", input.TerminalID),
	}, nil
}
