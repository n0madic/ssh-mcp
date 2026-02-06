package tools

import (
	"context"
	"fmt"

	"github.com/n0madic/ssh-mcp/internal/connection"
	"github.com/n0madic/ssh-mcp/internal/security"
)

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
