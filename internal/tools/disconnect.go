package tools

import (
	"context"
	"fmt"

	"github.com/n0madic/ssh-mcp/internal/connection"
)

// DisconnectDeps holds dependencies for the ssh_disconnect tool handler.
type DisconnectDeps struct {
	Pool *connection.Pool
}

// HandleDisconnect implements the ssh_disconnect tool.
func HandleDisconnect(_ context.Context, deps *DisconnectDeps, input SSHDisconnectInput) (*SSHDisconnectOutput, error) {
	sessionID := connection.SessionID(input.SessionID)

	if err := deps.Pool.Disconnect(sessionID); err != nil {
		return nil, fmt.Errorf("disconnect failed: %w", err)
	}

	return &SSHDisconnectOutput{
		Message: fmt.Sprintf("Disconnected session %s", input.SessionID),
	}, nil
}
