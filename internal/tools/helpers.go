package tools

import (
	"context"
	"fmt"

	"github.com/n0madic/ssh-mcp/internal/connection"
	"github.com/n0madic/ssh-mcp/internal/security"
)

// TruncateOutput truncates s to maxBytes and appends a truncation marker.
// If maxBytes <= 0 or len(s) <= maxBytes, s is returned unchanged.
func TruncateOutput(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	return s[:maxBytes] + fmt.Sprintf("\n[OUTPUT TRUNCATED: showing first %d of %d bytes]", maxBytes, len(s))
}

// getConnectionWithRateLimit retrieves a connection and optionally applies rate limiting.
// If rateLimiter is nil, rate limiting is skipped.
func getConnectionWithRateLimit(ctx context.Context, pool *connection.Pool, rateLimiter *security.RateLimiter, sessionID string) (*connection.Connection, error) {
	conn, err := pool.GetConnection(ctx, connection.SessionID(sessionID))
	if err != nil {
		return nil, fmt.Errorf("get connection: %w", err)
	}

	if rateLimiter != nil {
		if err := rateLimiter.Allow(conn.Host); err != nil {
			return nil, err
		}
	}

	return conn, nil
}
