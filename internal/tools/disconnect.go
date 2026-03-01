package tools

import (
	"context"
	"fmt"

	"github.com/n0madic/ssh-mcp/internal/connection"
	"github.com/n0madic/ssh-mcp/internal/tunnel"
)

// DisconnectDeps holds dependencies for the ssh_disconnect tool handler.
type DisconnectDeps struct {
	Pool       *connection.Pool
	TunnelPool *tunnel.TunnelPool
}

// HandleDisconnect implements the ssh_disconnect tool.
func HandleDisconnect(_ context.Context, deps *DisconnectDeps, input SSHDisconnectInput) (*SSHDisconnectOutput, error) {
	sessionID := connection.SessionID(input.SessionID)

	// Close all tunnels for this session before disconnecting.
	if deps.TunnelPool != nil {
		deps.TunnelPool.CloseBySession(input.SessionID)
	}

	if err := deps.Pool.Disconnect(sessionID); err != nil {
		return nil, fmt.Errorf("disconnect failed: %w", err)
	}

	return &SSHDisconnectOutput{
		Message: fmt.Sprintf("Disconnected session %s", input.SessionID),
	}, nil
}
