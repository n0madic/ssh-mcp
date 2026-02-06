package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/n0madic/ssh-mcp/internal/config"
)

func testConfig() *config.Config {
	return &config.Config{
		SSH: config.SSHConfig{
			KnownHostsPath:    "/nonexistent/known_hosts",
			VerifyHostKey:     false,
			ConfigPath:        "/nonexistent/ssh/config",
			KeySearchPaths:    []string{"/nonexistent/key"},
			CommandTimeout:    60 * time.Second,
			ConnectionTimeout: 30 * time.Second,
			MaxIdleTime:       5 * time.Minute,
			AllowSudo:         false,
			StripANSI:         true,
		},
		Security: config.SecurityConfig{
			RateLimit: 60,
		},
		Transport: config.TransportConfig{
			StdioEnabled: true,
			HTTPEnabled:  false,
			HTTPPort:     8081,
			HTTPPath:     "/mcp",
			HTTPHost:     "localhost",
		},
	}
}

func TestNew_CreatesServer(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()

	srv, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if srv.mcpServer == nil {
		t.Error("expected non-nil MCP server")
	}
	if srv.pool == nil {
		t.Error("expected non-nil pool")
	}
	if srv.filter == nil {
		t.Error("expected non-nil filter")
	}
	if srv.rateLimiter == nil {
		t.Error("expected non-nil rate limiter")
	}
}

func TestNew_InvalidFilter(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	cfg.Security.HostDenylist = []string{"[invalid-regex"}

	_, err := New(ctx, cfg)
	if err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestBoolPtr(t *testing.T) {
	truePtr := boolPtr(true)
	falsePtr := boolPtr(false)

	if *truePtr != true {
		t.Error("expected true")
	}
	if *falsePtr != false {
		t.Error("expected false")
	}
}

func TestAuthMiddleware_NoTokenConfigured(t *testing.T) {
	cfg := testConfig()
	cfg.Transport.HTTPToken = ""

	s := &Server{cfg: cfg}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	s.authMiddleware(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	cfg := testConfig()
	cfg.Transport.HTTPToken = "secret123"

	s := &Server{cfg: cfg}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer secret123")
	rec := httptest.NewRecorder()

	s.authMiddleware(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	cfg := testConfig()
	cfg.Transport.HTTPToken = "secret123"

	s := &Server{cfg: cfg}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rec := httptest.NewRecorder()

	s.authMiddleware(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	cfg := testConfig()
	cfg.Transport.HTTPToken = "secret123"

	s := &Server{cfg: cfg}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	s.authMiddleware(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}
