package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/n0madic/ssh-mcp/internal/tunnel"
)

func TestHandleTunnelCreate_MissingSessionID(t *testing.T) {
	deps := &TunnelDeps{
		TunnelPool: tunnel.NewTunnelPool(0),
	}
	_, err := HandleTunnelCreate(context.Background(), deps, SSHTunnelCreateInput{
		RemoteAddr: "localhost:5432",
	})
	if err == nil {
		t.Fatal("expected error for missing session_id")
	}
	if !strings.Contains(err.Error(), "session_id") {
		t.Errorf("expected error about session_id, got: %v", err)
	}
}

func TestHandleTunnelCreate_MissingRemoteAddr(t *testing.T) {
	deps := &TunnelDeps{
		TunnelPool: tunnel.NewTunnelPool(0),
	}
	_, err := HandleTunnelCreate(context.Background(), deps, SSHTunnelCreateInput{
		SessionID: "user@host:22",
	})
	if err == nil {
		t.Fatal("expected error for missing remote_addr")
	}
	if !strings.Contains(err.Error(), "remote_addr") {
		t.Errorf("expected error about remote_addr, got: %v", err)
	}
}

func TestHandleTunnelCreate_InvalidRemoteAddr(t *testing.T) {
	deps := &TunnelDeps{
		TunnelPool: tunnel.NewTunnelPool(0),
	}
	_, err := HandleTunnelCreate(context.Background(), deps, SSHTunnelCreateInput{
		SessionID:  "user@host:22",
		RemoteAddr: "not-a-host-port",
	})
	if err == nil {
		t.Fatal("expected error for invalid remote_addr format")
	}
	if !strings.Contains(err.Error(), "invalid remote_addr") {
		t.Errorf("expected error about invalid remote_addr, got: %v", err)
	}
}

func TestHandleTunnelClose_MissingTunnelID(t *testing.T) {
	deps := &TunnelDeps{
		TunnelPool: tunnel.NewTunnelPool(0),
	}
	_, err := HandleTunnelClose(context.Background(), deps, SSHTunnelCloseInput{})
	if err == nil {
		t.Fatal("expected error for missing tunnel_id")
	}
	if !strings.Contains(err.Error(), "tunnel_id") {
		t.Errorf("expected error about tunnel_id, got: %v", err)
	}
}

func TestHandleTunnelClose_NotFound(t *testing.T) {
	deps := &TunnelDeps{
		TunnelPool: tunnel.NewTunnelPool(0),
	}
	_, err := HandleTunnelClose(context.Background(), deps, SSHTunnelCloseInput{
		TunnelID: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for unknown tunnel")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error about not found, got: %v", err)
	}
}

func TestHandleTunnelList_Empty(t *testing.T) {
	deps := &TunnelDeps{
		TunnelPool: tunnel.NewTunnelPool(0),
	}
	out, err := HandleTunnelList(context.Background(), deps, SSHTunnelListInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Count != 0 {
		t.Errorf("expected 0 tunnels, got %d", out.Count)
	}
	if out.Text() != "No active tunnels" {
		t.Errorf("expected 'No active tunnels', got %q", out.Text())
	}
}

func TestSSHTunnelListOutput_Text(t *testing.T) {
	out := SSHTunnelListOutput{
		Tunnels: []TunnelInfoOutput{
			{
				TunnelID:   "test-1",
				LocalAddr:  "127.0.0.1:12345",
				RemoteAddr: "localhost:5432",
				ConnCount:  3,
				CreatedAt:  "2025-01-01T00:00:00Z",
			},
		},
		Count: 1,
	}
	text := out.Text()
	if !strings.Contains(text, "Active tunnels (1)") {
		t.Errorf("expected tunnel count in text, got: %s", text)
	}
	if !strings.Contains(text, "test-1") {
		t.Errorf("expected tunnel ID in text, got: %s", text)
	}
	if !strings.Contains(text, "127.0.0.1:12345") {
		t.Errorf("expected local addr in text, got: %s", text)
	}
}
