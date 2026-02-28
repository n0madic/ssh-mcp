package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/n0madic/ssh-mcp/internal/config"
	"github.com/n0madic/ssh-mcp/internal/connection"
)

// TestSpecialKeyMapping verifies that all documented special key names map to the
// correct byte sequences.
func TestSpecialKeyMapping(t *testing.T) {
	cases := []struct {
		name     string
		expected []byte
	}{
		{"CTRL_C", []byte{0x03}},
		{"CTRL_D", []byte{0x04}},
		{"CTRL_Z", []byte{0x1a}},
		{"ESC", []byte{0x1b}},
		{"TAB", []byte{0x09}},
		{"BACKSPACE", []byte{0x7f}},
		{"ENTER", []byte{'\r'}},
		{"ARROW_UP", []byte{0x1b, '[', 'A'}},
		{"ARROW_DOWN", []byte{0x1b, '[', 'B'}},
		{"ARROW_RIGHT", []byte{0x1b, '[', 'C'}},
		{"ARROW_LEFT", []byte{0x1b, '[', 'D'}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := specialKeys[tc.name]
			if !ok {
				t.Fatalf("special key %q not found in map", tc.name)
			}
			if len(got) != len(tc.expected) {
				t.Fatalf("length mismatch for %q: expected %d bytes, got %d", tc.name, len(tc.expected), len(got))
			}
			for i, b := range tc.expected {
				if got[i] != b {
					t.Errorf("byte %d for %q: expected 0x%02x, got 0x%02x", i, tc.name, b, got[i])
				}
			}
		})
	}
}

// TestSpecialKeyLookupInvalid verifies that the specialKeys map rejects unknown keys.
func TestSpecialKeyLookupInvalid(t *testing.T) {
	invalidKeys := []string{"INVALID_KEY", "ctrl_c", "F1", "DELETE", ""}
	for _, key := range invalidKeys {
		if _, ok := specialKeys[key]; ok {
			t.Errorf("expected specialKeys[%q] to be absent, but it was found", key)
		}
	}
}

// TestHandleOpenTerminalDisabledFlag verifies that HandleOpenTerminal returns an error
// when AllowTerminal is false (the default).
func TestHandleOpenTerminalDisabledFlag(t *testing.T) {
	deps := &TerminalDeps{
		Pool:     connection.NewPool(&config.SSHConfig{}, nil),
		TermPool: connection.NewTerminalPool(0),
		Config:   &config.SSHConfig{AllowTerminal: false},
	}

	_, err := HandleOpenTerminal(context.Background(), deps, SSHOpenTerminalInput{
		SessionID: "user@host:22",
	})
	if err == nil {
		t.Fatal("expected error when AllowTerminal=false, got nil")
	}
}

// TestHandleOpenTerminalMissingSession verifies that HandleOpenTerminal returns an error
// when session_id is empty.
func TestHandleOpenTerminalMissingSession(t *testing.T) {
	deps := &TerminalDeps{
		Pool:     connection.NewPool(&config.SSHConfig{}, nil),
		TermPool: connection.NewTerminalPool(0),
		Config:   &config.SSHConfig{AllowTerminal: true},
	}

	_, err := HandleOpenTerminal(context.Background(), deps, SSHOpenTerminalInput{})
	if err == nil {
		t.Fatal("expected error for empty session_id, got nil")
	}
}

// TestHandleSendInputMissingTerminal verifies that HandleSendInput returns an error
// for an unknown terminal ID.
func TestHandleSendInputMissingTerminal(t *testing.T) {
	deps := &TerminalDeps{
		Pool:     connection.NewPool(&config.SSHConfig{}, nil),
		TermPool: connection.NewTerminalPool(0),
		Config:   &config.SSHConfig{AllowTerminal: true},
	}

	_, err := HandleSendInput(context.Background(), deps, SSHSendInputInput{
		TerminalID: "term-999",
		Text:       "hello\n",
	})
	if err == nil {
		t.Fatal("expected error for unknown terminal, got nil")
	}
}

// TestHandleSendInputUnknownSpecialKey verifies the error returned for an invalid key name.
// The terminal ID must not exist so the error comes from pool.Get (not from key lookup).
// A separate test (TestSpecialKeyLookupInvalid) verifies the map rejects unknown keys.
func TestHandleSendInputUnknownSpecialKey(t *testing.T) {
	tp := connection.NewTerminalPool(0)

	deps := &TerminalDeps{
		Pool:     connection.NewPool(&config.SSHConfig{}, nil),
		TermPool: tp,
		Config:   &config.SSHConfig{AllowTerminal: true},
	}

	_, err := HandleSendInput(context.Background(), deps, SSHSendInputInput{
		TerminalID: "term-missing",
		SpecialKey: "INVALID_KEY",
	})
	if err == nil {
		t.Fatal("expected error for invalid special key, got nil")
	}
	// The error comes from pool.Get since "term-missing" doesn't exist.
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error to contain 'not found', got: %v", err)
	}
}

// TestHandleSendInputRequiresTextOrKey verifies that omitting both text and special_key
// returns an error.
func TestHandleSendInputRequiresTextOrKey(t *testing.T) {
	deps := &TerminalDeps{
		Pool:     connection.NewPool(&config.SSHConfig{}, nil),
		TermPool: connection.NewTerminalPool(0),
		Config:   &config.SSHConfig{AllowTerminal: true},
	}

	_, err := HandleSendInput(context.Background(), deps, SSHSendInputInput{
		TerminalID: "term-1",
	})
	if err == nil {
		t.Fatal("expected error when both text and special_key are empty, got nil")
	}
}

// TestHandleSendInputBothTextAndKey verifies that providing both text and special_key
// returns an error.
func TestHandleSendInputBothTextAndKey(t *testing.T) {
	deps := &TerminalDeps{
		Pool:     connection.NewPool(&config.SSHConfig{}, nil),
		TermPool: connection.NewTerminalPool(0),
		Config:   &config.SSHConfig{AllowTerminal: true},
	}

	_, err := HandleSendInput(context.Background(), deps, SSHSendInputInput{
		TerminalID: "term-1",
		Text:       "hello",
		SpecialKey: "ENTER",
	})
	if err == nil {
		t.Fatal("expected error when both text and special_key are set, got nil")
	}
	if !strings.Contains(err.Error(), "only one of") {
		t.Errorf("expected error about 'only one of', got: %v", err)
	}
}

// TestHandleReadOutputMissingTerminal verifies error for unknown terminal.
func TestHandleReadOutputMissingTerminal(t *testing.T) {
	deps := &TerminalDeps{
		Pool:     connection.NewPool(&config.SSHConfig{}, nil),
		TermPool: connection.NewTerminalPool(0),
		Config:   &config.SSHConfig{AllowTerminal: true},
	}

	_, err := HandleReadOutput(context.Background(), deps, SSHReadOutputInput{
		TerminalID: "term-999",
	})
	if err == nil {
		t.Fatal("expected error for unknown terminal, got nil")
	}
}

// TestEscapeReplacer verifies that escape sequences in text input are expanded correctly.
func TestEscapeReplacer(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{`hello\nworld`, "hello\nworld"},
		{`cmd\r\n`, "cmd\r\n"},
		{`col1\tcol2`, "col1\tcol2"},
		{`no escapes`, "no escapes"},
		{`\\n`, "\\\n"}, // backslash before \n: replacer still expands \n part
	}
	for _, tc := range cases {
		got := escapeReplacer.Replace(tc.input)
		if got != tc.expected {
			t.Errorf("escapeReplacer.Replace(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

// TestHandleCloseTerminalMissing verifies error for unknown terminal.
func TestHandleCloseTerminalMissing(t *testing.T) {
	deps := &TerminalDeps{
		Pool:     connection.NewPool(&config.SSHConfig{}, nil),
		TermPool: connection.NewTerminalPool(0),
		Config:   &config.SSHConfig{AllowTerminal: true},
	}

	_, err := HandleCloseTerminal(context.Background(), deps, SSHCloseTerminalInput{
		TerminalID: "term-999",
	})
	if err == nil {
		t.Fatal("expected error for unknown terminal, got nil")
	}
}
