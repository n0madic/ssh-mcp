package tunnel

import (
	"testing"
)

func TestNewTunnelPool(t *testing.T) {
	tp := NewTunnelPool(5)
	if tp == nil {
		t.Fatal("expected non-nil pool")
	}
	if tp.maxTunnels != 5 {
		t.Errorf("expected maxTunnels=5, got %d", tp.maxTunnels)
	}
}

func TestTunnelPool_OpenAndClose(t *testing.T) {
	tp := NewTunnelPool(0)

	// Open a tunnel with auto-assigned port (localPort=0).
	// We pass nil for the SSH client — the accept loop will run but
	// we must NOT connect to the listener (that would trigger a nil dereference
	// when trying to dial through the SSH client).
	ts, err := tp.Open("user@host:22", nil, 0, "localhost:5432")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ts.ID == "" {
		t.Error("expected non-empty tunnel ID")
	}
	if ts.LocalPort == 0 {
		t.Error("expected non-zero local port (auto-assigned)")
	}
	if ts.RemoteAddr != "localhost:5432" {
		t.Errorf("expected remote addr localhost:5432, got %s", ts.RemoteAddr)
	}
	if ts.SessionID != "user@host:22" {
		t.Errorf("expected session ID user@host:22, got %s", ts.SessionID)
	}

	// Verify the listener address is set.
	if ts.LocalAddr == "" {
		t.Error("expected non-empty local address")
	}

	// Verify Get works.
	got, err := tp.Get(ts.ID)
	if err != nil {
		t.Fatalf("unexpected error from Get: %v", err)
	}
	if got.ID != ts.ID {
		t.Errorf("expected ID %s, got %s", ts.ID, got.ID)
	}

	// Close the tunnel.
	if err := tp.Close(ts.ID); err != nil {
		t.Fatalf("unexpected error closing tunnel: %v", err)
	}

	// Verify it's removed.
	_, err = tp.Get(ts.ID)
	if err == nil {
		t.Error("expected error getting closed tunnel")
	}
}

func TestTunnelPool_GetUnknown(t *testing.T) {
	tp := NewTunnelPool(0)
	_, err := tp.Get("nonexistent")
	if err == nil {
		t.Error("expected error for unknown tunnel")
	}
}

func TestTunnelPool_CloseUnknown(t *testing.T) {
	tp := NewTunnelPool(0)
	err := tp.Close("nonexistent")
	if err == nil {
		t.Error("expected error closing unknown tunnel")
	}
}

func TestTunnelPool_MaxTunnels(t *testing.T) {
	tp := NewTunnelPool(1)

	ts1, err := tp.Open("user@host:22", nil, 0, "localhost:5432")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = tp.Open("user@host:22", nil, 0, "localhost:3306")
	if err == nil {
		t.Error("expected error when max tunnels reached")
	}

	// Close first tunnel, should allow a new one.
	tp.Close(ts1.ID)

	_, err = tp.Open("user@host:22", nil, 0, "localhost:3306")
	if err != nil {
		t.Fatalf("unexpected error after closing tunnel: %v", err)
	}
}

func TestTunnelPool_List(t *testing.T) {
	tp := NewTunnelPool(0)

	ts1, _ := tp.Open("session-1", nil, 0, "localhost:5432")
	ts2, _ := tp.Open("session-2", nil, 0, "localhost:3306")

	// List all.
	all := tp.List("")
	if len(all) != 2 {
		t.Errorf("expected 2 tunnels, got %d", len(all))
	}

	// List filtered by session.
	s1 := tp.List("session-1")
	if len(s1) != 1 {
		t.Errorf("expected 1 tunnel for session-1, got %d", len(s1))
	}
	if s1[0].ID != ts1.ID {
		t.Errorf("expected tunnel ID %s, got %s", ts1.ID, s1[0].ID)
	}

	s2 := tp.List("session-2")
	if len(s2) != 1 {
		t.Errorf("expected 1 tunnel for session-2, got %d", len(s2))
	}
	if s2[0].ID != ts2.ID {
		t.Errorf("expected tunnel ID %s, got %s", ts2.ID, s2[0].ID)
	}

	// List non-existent session.
	empty := tp.List("session-3")
	if len(empty) != 0 {
		t.Errorf("expected 0 tunnels for session-3, got %d", len(empty))
	}
}

func TestTunnelPool_CloseBySession(t *testing.T) {
	tp := NewTunnelPool(0)

	tp.Open("session-1", nil, 0, "localhost:5432")
	tp.Open("session-1", nil, 0, "localhost:3306")
	tp.Open("session-2", nil, 0, "localhost:6379")

	tp.CloseBySession("session-1")

	remaining := tp.List("")
	if len(remaining) != 1 {
		t.Errorf("expected 1 remaining tunnel, got %d", len(remaining))
	}
	if remaining[0].SessionID != "session-2" {
		t.Errorf("expected remaining tunnel to be session-2, got %s", remaining[0].SessionID)
	}
}

func TestTunnelPool_CloseAll(t *testing.T) {
	tp := NewTunnelPool(0)

	tp.Open("session-1", nil, 0, "localhost:5432")
	tp.Open("session-2", nil, 0, "localhost:3306")

	tp.CloseAll()

	all := tp.List("")
	if len(all) != 0 {
		t.Errorf("expected 0 tunnels after CloseAll, got %d", len(all))
	}
}

func TestTunnelPool_DoubleClose(t *testing.T) {
	tp := NewTunnelPool(0)

	ts, _ := tp.Open("session-1", nil, 0, "localhost:5432")

	// First close should succeed.
	if err := tp.Close(ts.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second close should return not found.
	if err := tp.Close(ts.ID); err == nil {
		t.Error("expected error on double close")
	}
}
