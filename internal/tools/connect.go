package tools

import (
	"context"
	"fmt"
	"os"

	"github.com/n0madic/ssh-mcp/internal/connection"
	"github.com/n0madic/ssh-mcp/internal/security"
)

// ConnectDeps holds dependencies for the ssh_connect tool handler.
type ConnectDeps struct {
	Pool        *connection.Pool
	Auth        *connection.AuthDiscovery
	Filter      *security.Filter
	RateLimiter *security.RateLimiter
}

// HandleConnect implements the ssh_connect tool.
func HandleConnect(ctx context.Context, deps *ConnectDeps, input SSHConnectInput) (*SSHConnectOutput, error) {
	// Parse host string (supports user:password@host:port format).
	params := connection.ParseHostString(input.Host)

	// Override with explicit parameters.
	if input.Port > 0 {
		params.Port = input.Port
	}
	if input.User != "" {
		params.User = input.User
	}
	if input.Password != "" {
		params.Password = input.Password
	}
	if input.KeyPath != "" {
		params.KeyPath = input.KeyPath
	}

	// Resolve from SSH config if requested.
	if input.UseSSHConfig {
		resolved := deps.Auth.ResolveHost(params.Host)
		if params.Host == input.Host { // not overridden by parsing
			params.Host = resolved.HostName
		}
		if input.Port == 0 && resolved.Port != 0 {
			params.Port = resolved.Port
		}
		if input.User == "" && resolved.User != "" {
			params.User = resolved.User
		}
		if input.KeyPath == "" && resolved.IdentityFile != "" {
			params.KeyPath = resolved.IdentityFile
		}
	}

	// Default user to current OS user.
	if params.User == "" {
		if u := os.Getenv("USER"); u != "" {
			params.User = u
		} else if u := os.Getenv("USERNAME"); u != "" {
			params.User = u
		} else {
			return nil, fmt.Errorf("no SSH user specified and could not determine current OS user; set USER env var or pass user explicitly")
		}
	}

	// Rate limit check.
	if err := deps.RateLimiter.Allow(params.Host); err != nil {
		return nil, err
	}

	// Host filter check.
	if err := deps.Filter.AllowHost(params.Host); err != nil {
		return nil, err
	}

	// Connect.
	sessionID, err := deps.Pool.Connect(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("connect failed: %w", err)
	}

	return &SSHConnectOutput{
		SessionID: string(sessionID),
		Host:      params.Host,
		Port:      params.Port,
		User:      params.User,
		Message:   fmt.Sprintf("Connected to %s@%s:%d", params.User, params.Host, params.Port),
	}, nil
}
