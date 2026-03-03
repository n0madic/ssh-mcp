package tools

import (
	"context"
	"fmt"
	"os"
	"os/user"

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
		if input.Port > 65535 {
			return nil, fmt.Errorf("invalid port: %d (must be 1-65535)", input.Port)
		}
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

	// Always resolve from SSH config (transparent alias discovery).
	parsedHost := params.Host // host after ParseHostString (without user@/:port)
	resolved := deps.Auth.ResolveHost(parsedHost)
	if params.Host == parsedHost { // not overridden by explicit input
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

	// Default user to current OS user.
	if params.User == "" {
		if currentUser, err := user.Current(); err == nil && currentUser.Username != "" {
			params.User = currentUser.Username
		} else if u := os.Getenv("USER"); u != "" {
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

	// Retrieve detected remote info.
	conn, err := deps.Pool.GetConnection(ctx, sessionID)
	if err != nil {
		// Connection succeeded but GetConnection failed — return basic output.
		return &SSHConnectOutput{
			SessionID: string(sessionID),
			Host:      params.Host,
			Port:      params.Port,
			User:      params.User,
			Message:   fmt.Sprintf("Connected to %s@%s:%d", params.User, params.Host, params.Port),
		}, nil
	}

	info := conn.GetRemoteInfo()
	message := fmt.Sprintf("Connected to %s@%s:%d", params.User, params.Host, params.Port)
	if info.OS != "" {
		detail := info.OS
		if info.Arch != "" {
			detail += " " + info.Arch
		}
		if info.Shell != "" {
			detail += ", " + info.Shell
		}
		message += fmt.Sprintf(" (%s)", detail)
	}

	return &SSHConnectOutput{
		SessionID: string(sessionID),
		Host:      params.Host,
		Port:      params.Port,
		User:      params.User,
		Message:   message,
		OS:        info.OS,
		Arch:      info.Arch,
		Shell:     info.Shell,
	}, nil
}
