package connection

import (
	"testing"
	"time"

	"github.com/n0madic/ssh-mcp/internal/config"
)

func newTestPool() *Pool {
	cfg := &config.SSHConfig{
		KeySearchPaths:    []string{"/nonexistent"},
		VerifyHostKey:     false,
		ConnectionTimeout: 5 * time.Second,
		MaxIdleTime:       5 * time.Minute,
	}
	auth := NewAuthDiscovery(cfg)
	return NewPool(cfg, auth)
}

func TestPool_ListConnections_Empty(t *testing.T) {
	pool := newTestPool()

	conns := pool.ListConnections()
	if len(conns) != 0 {
		t.Errorf("expected empty pool, got %d connections", len(conns))
	}
}

func TestPool_Disconnect_NotFound(t *testing.T) {
	pool := newTestPool()

	err := pool.Disconnect(SessionID("nonexistent"))
	if err == nil {
		t.Error("expected error for non-existent session")
	}
}

func TestPool_CloseAll_Empty(t *testing.T) {
	pool := newTestPool()

	// Should not panic on empty pool.
	pool.CloseAll()
}

func TestConnection_IncrementCommandCount(t *testing.T) {
	conn := &Connection{}
	conn.IncrementCommandCount()
	conn.IncrementCommandCount()
	conn.IncrementCommandCount()

	if conn.CommandCount != 3 {
		t.Errorf("expected command count 3, got %d", conn.CommandCount)
	}
}
