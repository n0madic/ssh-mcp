package tools

import (
	"context"
	"time"

	"github.com/n0madic/ssh-mcp/internal/connection"
	"github.com/n0madic/ssh-mcp/internal/tunnel"
)

// SessionsDeps holds dependencies for the ssh_list_sessions tool handler.
type SessionsDeps struct {
	Pool       *connection.Pool
	TermPool   *connection.TerminalPool
	TunnelPool *tunnel.TunnelPool
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
			OS:           c.OS,
			Arch:         c.Arch,
			Shell:        c.Shell,
		}

		// Include terminal sessions for this connection.
		if deps.TermPool != nil {
			infos := deps.TermPool.List(c.SessionID)
			if len(infos) > 0 {
				terminals := make([]TerminalInfoOutput, len(infos))
				for j, info := range infos {
					terminals[j] = TerminalInfoOutput{
						TerminalID: string(info.ID),
						SessionID:  string(info.SessionID),
						CreatedAt:  info.CreatedAt.Format(time.RFC3339),
						LastUsed:   info.LastUsed.Format(time.RFC3339),
					}
				}
				sessions[i].Terminals = terminals
			}
		}

		// Include tunnel sessions for this connection.
		if deps.TunnelPool != nil {
			tInfos := deps.TunnelPool.List(string(c.SessionID))
			if len(tInfos) > 0 {
				tunnels := make([]TunnelInfoOutput, len(tInfos))
				for j, info := range tInfos {
					tunnels[j] = tunnelInfoToOutput(info)
				}
				sessions[i].Tunnels = tunnels
			}
		}
	}

	return &SSHListSessionsOutput{
		Sessions: sessions,
		Count:    len(sessions),
	}, nil
}
