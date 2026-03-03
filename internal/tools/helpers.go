package tools

import (
	"context"
	"fmt"
	"unicode/utf8"

	"golang.org/x/crypto/ssh"

	"github.com/n0madic/ssh-mcp/internal/connection"
	"github.com/n0madic/ssh-mcp/internal/security"
)

// TruncateOutput truncates s to maxBytes and appends a truncation marker.
// If maxBytes <= 0 or len(s) <= maxBytes, s is returned unchanged.
// The truncation point is adjusted to avoid splitting a multi-byte UTF-8 character.
func TruncateOutput(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	end := maxBytes
	for end > 0 && !utf8.RuneStart(s[end]) {
		end--
	}
	return s[:end] + fmt.Sprintf("\n[OUTPUT TRUNCATED: showing first %d of %d bytes]", end, len(s))
}

// getConnectionWithRateLimit retrieves a connection and its SSH client, optionally applying rate limiting.
// If rateLimiter is nil, rate limiting is skipped.
func getConnectionWithRateLimit(ctx context.Context, pool *connection.Pool, rateLimiter *security.RateLimiter, sessionID string) (*connection.Connection, *ssh.Client, error) {
	conn, err := pool.GetConnection(ctx, connection.SessionID(sessionID))
	if err != nil {
		return nil, nil, fmt.Errorf("get connection: %w", err)
	}

	if rateLimiter != nil {
		if err := rateLimiter.Allow(conn.Host); err != nil {
			return nil, nil, err
		}
	}

	client, err := conn.GetClient()
	if err != nil {
		return nil, nil, err
	}

	return conn, client, nil
}
