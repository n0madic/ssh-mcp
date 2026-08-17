package tools

import (
	"strings"
	"testing"

	"github.com/n0madic/ssh-mcp/internal/connection"
)

func TestConnectMessage_IncludesSessionID(t *testing.T) {
	msg := connectMessage("user@example.com:22", connection.RemoteInfo{})
	if !strings.Contains(msg, "session_id: user@example.com:22") {
		t.Errorf("message must state the session_id explicitly, got: %q", msg)
	}
}

func TestConnectMessage_IncludesRemoteInfo(t *testing.T) {
	info := connection.RemoteInfo{
		OS:                 "Linux",
		Arch:               "x86_64",
		Shell:              "/bin/bash",
		PackageManager:     "apt",
		SudoNoninteractive: true,
	}
	msg := connectMessage("user@example.com:2222", info)

	want := "Connected, session_id: user@example.com:2222 (Linux x86_64, /bin/bash, pkg=apt, sudo-n)"
	if msg != want {
		t.Errorf("got %q, want %q", msg, want)
	}
}

func TestConnectMessage_PartialRemoteInfo(t *testing.T) {
	msg := connectMessage("user@example.com:22", connection.RemoteInfo{OS: "Linux"})
	want := "Connected, session_id: user@example.com:22 (Linux)"
	if msg != want {
		t.Errorf("got %q, want %q", msg, want)
	}
}
