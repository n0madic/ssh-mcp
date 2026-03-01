package tools

import (
	"context"
	"fmt"

	"github.com/n0madic/ssh-mcp/internal/connection"
	"github.com/n0madic/ssh-mcp/internal/security"
	"github.com/n0madic/ssh-mcp/internal/tunnel"
)

// TunnelDeps holds dependencies for tunnel tool handlers.
type TunnelDeps struct {
	Pool        *connection.Pool
	TunnelPool  *tunnel.TunnelPool
	RateLimiter *security.RateLimiter
}

// HandleTunnelCreate creates a new local port forwarding tunnel.
func HandleTunnelCreate(ctx context.Context, deps *TunnelDeps, input SSHTunnelCreateInput) (*SSHTunnelCreateOutput, error) {
	if input.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}
	if input.RemoteAddr == "" {
		return nil, fmt.Errorf("remote_addr is required")
	}

	conn, err := getConnectionWithRateLimit(ctx, deps.Pool, deps.RateLimiter, input.SessionID)
	if err != nil {
		return nil, err
	}

	ts, err := deps.TunnelPool.Open(input.SessionID, conn.Client, input.LocalPort, input.RemoteAddr)
	if err != nil {
		return nil, fmt.Errorf("create tunnel: %w", err)
	}

	return &SSHTunnelCreateOutput{
		TunnelID:   string(ts.ID),
		LocalAddr:  ts.LocalAddr,
		LocalPort:  ts.LocalPort,
		RemoteAddr: ts.RemoteAddr,
		Message:    fmt.Sprintf("Tunnel created: %s → %s (listening on %s)", ts.LocalAddr, ts.RemoteAddr, ts.LocalAddr),
	}, nil
}

// HandleTunnelList lists active tunnels, optionally filtered by session ID.
func HandleTunnelList(_ context.Context, deps *TunnelDeps, input SSHTunnelListInput) (*SSHTunnelListOutput, error) {
	infos := deps.TunnelPool.List(input.SessionID)

	tunnels := make([]TunnelInfoOutput, len(infos))
	for i, info := range infos {
		tunnels[i] = tunnelInfoToOutput(info)
	}

	return &SSHTunnelListOutput{
		Tunnels: tunnels,
		Count:   len(tunnels),
	}, nil
}

// HandleTunnelClose closes an active tunnel.
func HandleTunnelClose(_ context.Context, deps *TunnelDeps, input SSHTunnelCloseInput) (*SSHTunnelCloseOutput, error) {
	if input.TunnelID == "" {
		return nil, fmt.Errorf("tunnel_id is required")
	}

	if err := deps.TunnelPool.Close(tunnel.TunnelID(input.TunnelID)); err != nil {
		return nil, err
	}

	return &SSHTunnelCloseOutput{
		Message: fmt.Sprintf("Tunnel %s closed", input.TunnelID),
	}, nil
}

// tunnelInfoToOutput converts a tunnel.TunnelInfo to TunnelInfoOutput.
func tunnelInfoToOutput(info tunnel.TunnelInfo) TunnelInfoOutput {
	return TunnelInfoOutput{
		TunnelID:   string(info.ID),
		SessionID:  info.SessionID,
		LocalAddr:  info.LocalAddr,
		RemoteAddr: info.RemoteAddr,
		ConnCount:  info.ConnCount,
		CreatedAt:  info.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		LastUsed:   info.LastUsed.Format("2006-01-02T15:04:05Z07:00"),
	}
}
