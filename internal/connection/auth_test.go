package connection

import (
	"strings"
	"testing"
	"time"

	"github.com/n0madic/ssh-mcp/internal/config"
)

func TestParseHostString_Simple(t *testing.T) {
	params := ParseHostString("example.com")
	if params.Host != "example.com" {
		t.Errorf("expected host=example.com, got %s", params.Host)
	}
	if params.Port != 22 {
		t.Errorf("expected port=22, got %d", params.Port)
	}
	if params.User != "" {
		t.Errorf("expected empty user, got %s", params.User)
	}
}

func TestParseHostString_WithPort(t *testing.T) {
	params := ParseHostString("example.com:2222")
	if params.Host != "example.com" {
		t.Errorf("expected host=example.com, got %s", params.Host)
	}
	if params.Port != 2222 {
		t.Errorf("expected port=2222, got %d", params.Port)
	}
}

func TestParseHostString_WithUser(t *testing.T) {
	params := ParseHostString("admin@example.com")
	if params.Host != "example.com" {
		t.Errorf("expected host=example.com, got %s", params.Host)
	}
	if params.User != "admin" {
		t.Errorf("expected user=admin, got %s", params.User)
	}
}

func TestParseHostString_WithUserAndPort(t *testing.T) {
	params := ParseHostString("admin@example.com:2222")
	if params.Host != "example.com" {
		t.Errorf("expected host=example.com, got %s", params.Host)
	}
	if params.Port != 2222 {
		t.Errorf("expected port=2222, got %d", params.Port)
	}
	if params.User != "admin" {
		t.Errorf("expected user=admin, got %s", params.User)
	}
}

func TestParseHostString_WithPassword(t *testing.T) {
	params := ParseHostString("admin:secret@example.com:2222")
	if params.Host != "example.com" {
		t.Errorf("expected host=example.com, got %s", params.Host)
	}
	if params.Port != 2222 {
		t.Errorf("expected port=2222, got %d", params.Port)
	}
	if params.User != "admin" {
		t.Errorf("expected user=admin, got %s", params.User)
	}
	if params.Password != "secret" {
		t.Errorf("expected password=secret, got %s", params.Password)
	}
}

func TestMakeSessionID(t *testing.T) {
	id := MakeSessionID("user", "host.com", 22)
	if string(id) != "user@host.com:22" {
		t.Errorf("expected user@host.com:22, got %s", id)
	}
}

func TestAuthDiscovery_BuildAuthMethods_NoKeys(t *testing.T) {
	cfg := &config.SSHConfig{
		KeySearchPaths:    []string{"/nonexistent/path"},
		ConnectionTimeout: 30 * time.Second,
	}
	auth := NewAuthDiscovery(cfg)

	params := ConnectParams{
		Password: "test",
	}
	methods := auth.BuildAuthMethods(params)
	if len(methods) != 1 {
		t.Errorf("expected 1 auth method (password), got %d", len(methods))
	}
}

func TestAuthDiscovery_BuildAuthMethods_NoMethods(t *testing.T) {
	cfg := &config.SSHConfig{
		KeySearchPaths:    []string{"/nonexistent/path"},
		ConnectionTimeout: 30 * time.Second,
	}
	auth := NewAuthDiscovery(cfg)

	params := ConnectParams{}
	methods := auth.BuildAuthMethods(params)
	if len(methods) != 0 {
		t.Errorf("expected 0 auth methods, got %d", len(methods))
	}
}

func TestAuthDiscovery_BuildClientConfig_NoMethods(t *testing.T) {
	cfg := &config.SSHConfig{
		KeySearchPaths:    []string{"/nonexistent/path"},
		VerifyHostKey:     false,
		ConnectionTimeout: 30 * time.Second,
	}
	auth := NewAuthDiscovery(cfg)

	params := ConnectParams{}
	_, err := auth.BuildClientConfig(params)
	if err == nil {
		t.Error("expected error when no auth methods available")
	}
}

func TestAuthDiscovery_ResolveHost_NoConfig(t *testing.T) {
	cfg := &config.SSHConfig{
		ConfigPath:        "/nonexistent/ssh/config",
		ConnectionTimeout: 30 * time.Second,
	}
	auth := NewAuthDiscovery(cfg)

	resolved := auth.ResolveHost("myhost")
	if resolved.HostName != "myhost" {
		t.Errorf("expected hostname=myhost, got %s", resolved.HostName)
	}
	if resolved.Port != 22 {
		t.Errorf("expected port=22, got %d", resolved.Port)
	}
}

func TestBuildHostKeyCallback_MissingKnownHosts(t *testing.T) {
	cfg := &config.SSHConfig{
		KnownHostsPath:    "/nonexistent/known_hosts",
		VerifyHostKey:     true,
		KeySearchPaths:    []string{"/nonexistent/key"},
		ConnectionTimeout: 30 * time.Second,
	}
	auth := NewAuthDiscovery(cfg)

	params := ConnectParams{
		Password: "test",
	}
	_, err := auth.BuildClientConfig(params)
	if err == nil {
		t.Error("expected error when known_hosts missing")
	}
	if !strings.Contains(err.Error(), "known_hosts") {
		t.Errorf("expected error message to mention known_hosts, got: %v", err)
	}
}
