package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	// Temporarily clear env vars to not interfere with defaults.
	for _, env := range []string{
		"MCP_SSH_ENABLE_HTTP", "MCP_SSH_HTTP_PORT", "MCP_SSH_DISABLE_STDIO",
		"MCP_SSH_NO_VERIFY_HOST_KEY", "MCP_SSH_ENABLE_SUDO", "MCP_SSH_RATE_LIMIT",
	} {
		if v, ok := os.LookupEnv(env); ok {
			os.Unsetenv(env)
			defer os.Setenv(env, v)
		}
	}

	args := Args{
		HTTPPort:       8081,
		CommandTimeout: 60 * time.Second,
		RateLimit:      60,
	}
	cfg := buildConfig(args)

	if cfg.SSH.VerifyHostKey != true {
		t.Error("expected VerifyHostKey to be true by default")
	}
	if cfg.SSH.AllowSudo != false {
		t.Error("expected AllowSudo to be false by default")
	}
	if cfg.SSH.StripANSI != true {
		t.Error("expected StripANSI to be true by default")
	}
	if cfg.SSH.CommandTimeout != 60*time.Second {
		t.Errorf("expected CommandTimeout=60s, got %v", cfg.SSH.CommandTimeout)
	}
	if cfg.SSH.ConnectionTimeout != 30*time.Second {
		t.Errorf("expected ConnectionTimeout=30s, got %v", cfg.SSH.ConnectionTimeout)
	}
	if cfg.SSH.MaxIdleTime != 5*time.Minute {
		t.Errorf("expected MaxIdleTime=5m, got %v", cfg.SSH.MaxIdleTime)
	}
	if cfg.Transport.StdioEnabled != true {
		t.Error("expected StdioEnabled to be true by default")
	}
	if cfg.Transport.HTTPEnabled != false {
		t.Error("expected HTTPEnabled to be false by default")
	}
	if cfg.Transport.HTTPPort != 8081 {
		t.Errorf("expected HTTPPort=8081, got %d", cfg.Transport.HTTPPort)
	}
	if cfg.Transport.HTTPHost != "localhost" {
		t.Errorf("expected HTTPHost=localhost, got %s", cfg.Transport.HTTPHost)
	}
	if cfg.Transport.HTTPPath != "/mcp" {
		t.Errorf("expected HTTPPath=/mcp, got %s", cfg.Transport.HTTPPath)
	}
	if cfg.Security.RateLimit != 60 {
		t.Errorf("expected RateLimit=60, got %d", cfg.Security.RateLimit)
	}
	if len(cfg.SSH.KeySearchPaths) != 4 {
		t.Errorf("expected 4 key search paths, got %d", len(cfg.SSH.KeySearchPaths))
	}
}

func TestBuildConfig_HTTPEnabled(t *testing.T) {
	args := Args{
		EnableHTTP:     true,
		HTTPPort:       9090,
		CommandTimeout: 60 * time.Second,
		RateLimit:      60,
	}
	cfg := buildConfig(args)

	if !cfg.Transport.HTTPEnabled {
		t.Error("expected HTTPEnabled=true")
	}
	if cfg.Transport.HTTPPort != 9090 {
		t.Errorf("expected HTTPPort=9090, got %d", cfg.Transport.HTTPPort)
	}
	// Host must always be localhost.
	if cfg.Transport.HTTPHost != "localhost" {
		t.Errorf("expected HTTPHost=localhost, got %s", cfg.Transport.HTTPHost)
	}
}

func TestBuildConfig_DisableStdio(t *testing.T) {
	args := Args{
		DisableStdio:   true,
		EnableHTTP:     true,
		HTTPPort:       8081,
		CommandTimeout: 60 * time.Second,
		RateLimit:      60,
	}
	cfg := buildConfig(args)

	if cfg.Transport.StdioEnabled {
		t.Error("expected StdioEnabled=false when disabled")
	}
}

func TestBuildConfig_NoVerifyHost(t *testing.T) {
	args := Args{
		NoVerifyHost:   true,
		HTTPPort:       8081,
		CommandTimeout: 60 * time.Second,
		RateLimit:      60,
	}
	cfg := buildConfig(args)

	if cfg.SSH.VerifyHostKey {
		t.Error("expected VerifyHostKey=false when --no-verify-host-key")
	}
}

func TestBuildConfig_EnableSudo(t *testing.T) {
	args := Args{
		EnableSudo:     true,
		HTTPPort:       8081,
		CommandTimeout: 60 * time.Second,
		RateLimit:      60,
	}
	cfg := buildConfig(args)

	if !cfg.SSH.AllowSudo {
		t.Error("expected AllowSudo=true when --enable-sudo")
	}
}

func TestBuildConfig_SecurityLists(t *testing.T) {
	args := Args{
		HostAllowlist:    commaSeparated{"host1", "host2"},
		HostDenylist:     commaSeparated{"bad-host"},
		CommandDenylist:  commaSeparated{"rm -rf", "shutdown"},
		CommandAllowlist: commaSeparated{"ls", "cat"},
		HTTPPort:         8081,
		CommandTimeout:   60 * time.Second,
		RateLimit:        60,
	}
	cfg := buildConfig(args)

	if len(cfg.Security.HostAllowlist) != 2 {
		t.Errorf("expected 2 host allowlist entries, got %d", len(cfg.Security.HostAllowlist))
	}
	if len(cfg.Security.HostDenylist) != 1 {
		t.Errorf("expected 1 host denylist entry, got %d", len(cfg.Security.HostDenylist))
	}
	if len(cfg.Security.CommandDenylist) != 2 {
		t.Errorf("expected 2 command denylist entries, got %d", len(cfg.Security.CommandDenylist))
	}
	if len(cfg.Security.CommandAllowlist) != 2 {
		t.Errorf("expected 2 command allowlist entries, got %d", len(cfg.Security.CommandAllowlist))
	}
}

func TestValidate_Valid(t *testing.T) {
	args := Args{
		HTTPPort:       8081,
		CommandTimeout: 60 * time.Second,
		RateLimit:      60,
	}
	cfg := buildConfig(args)
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	args := Args{
		HTTPPort:       0,
		CommandTimeout: 60 * time.Second,
		RateLimit:      60,
	}
	cfg := buildConfig(args)
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for port 0")
	}
}

func TestValidate_NoTransport(t *testing.T) {
	args := Args{
		DisableStdio:   true,
		EnableHTTP:     false,
		HTTPPort:       8081,
		CommandTimeout: 60 * time.Second,
		RateLimit:      60,
	}
	cfg := buildConfig(args)
	if err := cfg.Validate(); err == nil {
		t.Error("expected error when no transport enabled")
	}
}

func TestValidate_InvalidTimeout(t *testing.T) {
	args := Args{
		HTTPPort:       8081,
		CommandTimeout: -1 * time.Second,
		RateLimit:      60,
	}
	cfg := buildConfig(args)
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for negative timeout")
	}
}

func TestValidate_InvalidRateLimit(t *testing.T) {
	args := Args{
		HTTPPort:       8081,
		CommandTimeout: 60 * time.Second,
		RateLimit:      0,
	}
	cfg := buildConfig(args)
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for zero rate limit")
	}
}

func TestCommaSeparated_UnmarshalText(t *testing.T) {
	var c commaSeparated

	// Test comma-separated values
	if err := c.UnmarshalText([]byte("host1,host2,host3")); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(c) != 3 {
		t.Errorf("expected 3 values, got %d", len(c))
	}
	if c[0] != "host1" || c[1] != "host2" || c[2] != "host3" {
		t.Errorf("unexpected values: %v", c)
	}

	// Test with spaces
	c = nil
	if err := c.UnmarshalText([]byte("  host1  ,  host2  ,  host3  ")); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(c) != 3 {
		t.Errorf("expected 3 values, got %d", len(c))
	}
	if c[0] != "host1" || c[1] != "host2" || c[2] != "host3" {
		t.Errorf("unexpected values: %v", c)
	}

	// Test empty values filtered out
	c = nil
	if err := c.UnmarshalText([]byte("host1,,host2,  ,host3")); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(c) != 3 {
		t.Errorf("expected 3 values (empty filtered), got %d", len(c))
	}

	// Test single value
	c = nil
	if err := c.UnmarshalText([]byte("single")); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(c) != 1 || c[0] != "single" {
		t.Errorf("unexpected value: %v", c)
	}
}

func TestBuildConfig_NewSecurityFlags(t *testing.T) {
	args := Args{
		LocalBaseDir:     "/tmp",
		MaxFileSize:      1048576,
		MaxConnections:   5,
		HTTPToken:        "secret123",
		RateLimitFileOps: true,
		HTTPPort:         8081,
		CommandTimeout:   60 * time.Second,
		RateLimit:        60,
	}
	cfg := buildConfig(args)

	if cfg.Security.LocalBaseDir != "/tmp" {
		t.Errorf("expected LocalBaseDir=/tmp, got %s", cfg.Security.LocalBaseDir)
	}
	if cfg.Security.MaxFileSize != 1048576 {
		t.Errorf("expected MaxFileSize=1048576, got %d", cfg.Security.MaxFileSize)
	}
	if cfg.SSH.MaxConnections != 5 {
		t.Errorf("expected MaxConnections=5, got %d", cfg.SSH.MaxConnections)
	}
	if cfg.Transport.HTTPToken != "secret123" {
		t.Errorf("expected HTTPToken=secret123, got %s", cfg.Transport.HTTPToken)
	}
	if !cfg.Security.RateLimitFileOps {
		t.Error("expected RateLimitFileOps=true")
	}
}

func TestValidate_InvalidMaxFileSize(t *testing.T) {
	args := Args{
		MaxFileSize:    -1,
		HTTPPort:       8081,
		CommandTimeout: 60 * time.Second,
		RateLimit:      60,
	}
	cfg := buildConfig(args)
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for negative max file size")
	}
}

func TestValidate_InvalidMaxConnections(t *testing.T) {
	args := Args{
		HTTPPort:       8081,
		CommandTimeout: 60 * time.Second,
		RateLimit:      60,
	}
	cfg := buildConfig(args)
	cfg.SSH.MaxConnections = -1
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for negative max connections")
	}
}
