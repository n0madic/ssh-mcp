package tools

import (
	"context"
	"time"

	"github.com/n0madic/ssh-mcp/internal/connection"
)

// SessionsDeps holds dependencies for the ssh_list_sessions tool handler.
type SessionsDeps struct {
	Pool *connection.Pool
}

// SSHListSessionsInput is the input for ssh_list_sessions (empty, no parameters needed).
type SSHListSessionsInput struct{}

// HandleListSessions implements the ssh_list_sessions tool.
// Access control: when HTTP transport is used, access is gated by the --http-token bearer auth middleware.
func HandleListSessions(_ context.Context, deps *SessionsDeps, _ SSHListSessionsInput) (*SSHListSessionsOutput, error) {
	conns := deps.Pool.ListConnections()

	sessions := make([]SessionInfo, len(conns))
	for i, c := range conns {
		sessions[i] = SessionInfo{
			SessionID:    string(c.SessionID),
			Host:         c.Host,
			Port:         c.Port,
			User:         c.User,
			ConnectedAt:  c.ConnectedAt.Format(time.RFC3339),
			LastUsed:     c.LastUsed.Format(time.RFC3339),
			CommandCount: c.CommandCount,
			Connected:    c.Connected,
		}
	}

	return &SSHListSessionsOutput{
		Sessions: sessions,
		Count:    len(sessions),
	}, nil
}
