package connection

import (
	"context"
	"fmt"
	"strings"
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

func TestPool_GetConnection_WaitsPendingConnect(t *testing.T) {
	pool := newTestPool()
	id := SessionID("user@example.com:22")

	// Create a pending connection (ready channel open).
	pending := &Connection{
		ID:    id,
		Host:  "example.com",
		Port:  22,
		User:  "user",
		ready: make(chan struct{}),
	}

	pool.mu.Lock()
	pool.conns[id] = pending
	pool.mu.Unlock()

	ctx := context.Background()
	done := make(chan error, 1)

	// GetConnection should block until ready is closed.
	go func() {
		_, err := pool.GetConnection(ctx, id)
		done <- err
	}()

	// Verify it hasn't returned yet.
	select {
	case <-done:
		t.Fatal("GetConnection returned before connection was ready")
	case <-time.After(50 * time.Millisecond):
		// Expected: still waiting.
	}

	// Simulate successful connection.
	pending.mu.Lock()
	pending.Connected = true
	pending.LastUsed = time.Now()
	pending.mu.Unlock()
	close(pending.ready)

	// Now GetConnection should return (may fail on isAlive since no real SSH client,
	// but it should not return "session not found").
	select {
	case err := <-done:
		if err != nil && strings.Contains(err.Error(), "not found") {
			t.Fatalf("unexpected 'not found' error: %v", err)
		}
		// Other errors (like isAlive failing) are expected without a real SSH client.
	case <-time.After(2 * time.Second):
		t.Fatal("GetConnection timed out after ready was signaled")
	}
}

func TestPool_GetConnection_PendingConnectFails(t *testing.T) {
	pool := newTestPool()
	id := SessionID("user@fail.com:22")

	pending := &Connection{
		ID:    id,
		Host:  "fail.com",
		Port:  22,
		User:  "user",
		ready: make(chan struct{}),
	}

	pool.mu.Lock()
	pool.conns[id] = pending
	pool.mu.Unlock()

	ctx := context.Background()
	done := make(chan error, 1)

	go func() {
		_, err := pool.GetConnection(ctx, id)
		done <- err
	}()

	// Simulate failed connection.
	pending.connectErr = fmt.Errorf("SSH dial fail.com:22: connection refused")
	close(pending.ready)

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error from failed connection, got nil")
		}
		if !strings.Contains(err.Error(), "connection failed") {
			t.Fatalf("expected 'connection failed' error, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("GetConnection timed out after ready was signaled")
	}
}

func TestPool_GetConnection_ContextCancelled(t *testing.T) {
	pool := newTestPool()
	id := SessionID("user@slow.com:22")

	pending := &Connection{
		ID:    id,
		Host:  "slow.com",
		Port:  22,
		User:  "user",
		ready: make(chan struct{}),
	}

	pool.mu.Lock()
	pool.conns[id] = pending
	pool.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		_, err := pool.GetConnection(ctx, id)
		done <- err
	}()

	// Cancel context while connection is still pending.
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error from cancelled context, got nil")
		}
		if err != context.Canceled {
			t.Fatalf("expected context.Canceled, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("GetConnection did not unblock after context cancellation")
	}
}

func TestPool_ListConnections_PendingShownAsDisconnected(t *testing.T) {
	pool := newTestPool()

	// Add a pending connection.
	pending := &Connection{
		ID:    SessionID("user@pending.com:22"),
		Host:  "pending.com",
		Port:  22,
		User:  "user",
		ready: make(chan struct{}),
	}
	pool.mu.Lock()
	pool.conns[pending.ID] = pending
	pool.mu.Unlock()

	infos := pool.ListConnections()
	if len(infos) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(infos))
	}
	if infos[0].Connected {
		t.Error("pending connection should be reported as not connected")
	}
	if infos[0].Host != "pending.com" {
		t.Errorf("expected host pending.com, got %s", infos[0].Host)
	}
}

func TestPool_Disconnect_WaitsPending(t *testing.T) {
	pool := newTestPool()
	id := SessionID("user@disc.com:22")

	pending := &Connection{
		ID:    id,
		Host:  "disc.com",
		Port:  22,
		User:  "user",
		ready: make(chan struct{}),
	}
	pool.mu.Lock()
	pool.conns[id] = pending
	pool.mu.Unlock()

	done := make(chan error, 1)
	go func() {
		done <- pool.Disconnect(id)
	}()

	// Verify Disconnect blocks while pending.
	select {
	case <-done:
		t.Fatal("Disconnect returned before connection was ready")
	case <-time.After(50 * time.Millisecond):
	}

	// Complete the pending connection (no real client).
	close(pending.ready)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Disconnect timed out after ready was signaled")
	}
}
