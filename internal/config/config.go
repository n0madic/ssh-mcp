package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexflint/go-arg"
)

// Version is set at build time via ldflags.
var Version = "dev"

// commaSeparated is a custom type for parsing comma-separated lists.
// Supports both repeated flags (--flag val1 --flag val2) and
// comma-separated env vars (VAR="val1,val2,val3").
type commaSeparated []string

func (c *commaSeparated) UnmarshalText(b []byte) error {
	parts := strings.Split(string(b), ",")
	result := make([]string, 0, len(parts))
	for _, s := range parts {
		s = strings.TrimSpace(s)
		if s != "" {
			result = append(result, s)
		}
	}
	*c = result
	return nil
}

// Args holds CLI arguments parsed by go-arg.
type Args struct {
	EnableHTTP       bool           `arg:"--enable-http,env:MCP_SSH_ENABLE_HTTP" help:"enable HTTP transport"`
	HTTPPort         int            `arg:"--http-port,env:MCP_SSH_HTTP_PORT" default:"8081" placeholder:"PORT" help:"HTTP transport port"`
	DisableStdio     bool           `arg:"--disable-stdio,env:MCP_SSH_DISABLE_STDIO" help:"disable stdio transport"`
	NoVerifyHost     bool           `arg:"--no-verify-host-key,env:MCP_SSH_NO_VERIFY_HOST_KEY" help:"disable host key verification"`
	KnownHosts       string         `arg:"--known-hosts,env:MCP_SSH_KNOWN_HOSTS" placeholder:"PATH" help:"path to known_hosts file"`
	SSHConfigPath    string         `arg:"--ssh-config,env:MCP_SSH_CONFIG" placeholder:"PATH" help:"path to SSH config file"`
	EnableSudo       bool           `arg:"--enable-sudo,env:MCP_SSH_ENABLE_SUDO" help:"allow sudo execution"`
	CommandTimeout   time.Duration  `arg:"--command-timeout,env:MCP_SSH_COMMAND_TIMEOUT" default:"60s" placeholder:"DURATION" help:"command execution timeout"`
	HostAllowlist    commaSeparated `arg:"--host-allowlist,separate,env:MCP_SSH_HOST_ALLOWLIST" placeholder:"PATTERN" help:"host allowlist (can be specified multiple times or comma-separated)"`
	HostDenylist     commaSeparated `arg:"--host-denylist,separate,env:MCP_SSH_HOST_DENYLIST" placeholder:"PATTERN" help:"host denylist (can be specified multiple times or comma-separated)"`
	CommandAllowlist commaSeparated `arg:"--command-allowlist,separate,env:MCP_SSH_COMMAND_ALLOWLIST" placeholder:"REGEX" help:"command allowlist regex (can be specified multiple times or comma-separated)"`
	CommandDenylist  commaSeparated `arg:"--command-denylist,separate,env:MCP_SSH_COMMAND_DENYLIST" placeholder:"REGEX" help:"command denylist regex (can be specified multiple times or comma-separated)"`
	RateLimit        int            `arg:"--rate-limit,env:MCP_SSH_RATE_LIMIT" default:"60" placeholder:"NUM" help:"rate limit (requests per minute)"`
	RateLimitFileOps bool           `arg:"--rate-limit-file-ops,env:MCP_SSH_RATE_LIMIT_FILE_OPS" help:"apply rate limiting to SFTP file operations"`
	LocalBaseDir     string         `arg:"--local-base-dir,env:MCP_SSH_LOCAL_BASE_DIR" placeholder:"PATH" help:"restrict local file operations to this directory"`
	MaxFileSize      int64          `arg:"--max-file-size,env:MCP_SSH_MAX_FILE_SIZE" default:"0" placeholder:"BYTES" help:"maximum file size for read operations (0=unlimited)"`
	MaxConnections   int            `arg:"--max-connections,env:MCP_SSH_MAX_CONNECTIONS" default:"0" placeholder:"NUM" help:"maximum number of concurrent SSH connections (0=unlimited)"`
	HTTPToken        string         `arg:"--http-token,env:MCP_SSH_HTTP_TOKEN" placeholder:"TOKEN" help:"bearer token for HTTP transport authentication"`
	DisableTools     commaSeparated `arg:"--disable-tools,separate,env:MCP_SSH_DISABLE_TOOLS" placeholder:"TOOL" help:"disable specific tools (can be specified multiple times or comma-separated)"`
	ShowVersion      bool           `arg:"--version" help:"show version and exit"`
}

// Description returns the program description for go-arg.
func (Args) Description() string {
	return "SSH MCP Server - provides AI agents with secure SSH access to remote hosts"
}

// Version returns the version string for go-arg.
func (Args) Version() string {
	return "ssh-mcp " + Version
}

// Config holds all configuration for the SSH MCP server.
type Config struct {
	SSH           SSHConfig
	Security      SecurityConfig
	Transport     TransportConfig
	DisabledTools []string
}

// SSHConfig holds SSH-related configuration.
type SSHConfig struct {
	KnownHostsPath    string
	VerifyHostKey     bool
	ConfigPath        string
	KeySearchPaths    []string
	CommandTimeout    time.Duration
	ConnectionTimeout time.Duration
	MaxIdleTime       time.Duration
	AllowSudo         bool
	StripANSI         bool
	MaxConnections    int
}

// SecurityConfig holds security-related configuration.
type SecurityConfig struct {
	HostAllowlist    []string
	HostDenylist     []string
	CommandAllowlist []string
	CommandDenylist  []string
	RateLimit        int // requests per minute
	RateLimitFileOps bool
	LocalBaseDir     string
	MaxFileSize      int64
}

// TransportConfig holds transport-related configuration.
type TransportConfig struct {
	StdioEnabled bool
	HTTPEnabled  bool
	HTTPPort     int
	HTTPPath     string
	HTTPHost     string // always "localhost", not configurable
	HTTPToken    string
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	if c.Transport.HTTPPort < 1 || c.Transport.HTTPPort > 65535 {
		return fmt.Errorf("invalid HTTP port: %d", c.Transport.HTTPPort)
	}
	if !c.Transport.StdioEnabled && !c.Transport.HTTPEnabled {
		return fmt.Errorf("at least one transport (stdio or HTTP) must be enabled")
	}
	if c.SSH.CommandTimeout <= 0 {
		return fmt.Errorf("command timeout must be positive")
	}
	if c.SSH.ConnectionTimeout <= 0 {
		return fmt.Errorf("connection timeout must be positive")
	}
	if c.Security.RateLimit <= 0 {
		return fmt.Errorf("rate limit must be positive")
	}
	if c.Security.LocalBaseDir != "" {
		absPath, err := filepath.Abs(c.Security.LocalBaseDir)
		if err != nil {
			return fmt.Errorf("invalid local base dir: %w", err)
		}
		if info, err := os.Stat(absPath); err != nil || !info.IsDir() {
			return fmt.Errorf("local base dir %q does not exist or is not a directory", c.Security.LocalBaseDir)
		}
		c.Security.LocalBaseDir = absPath
	}
	if c.Security.MaxFileSize < 0 {
		return fmt.Errorf("max file size must be non-negative")
	}
	if c.SSH.MaxConnections < 0 {
		return fmt.Errorf("max connections must be non-negative")
	}
	return nil
}

// Parse parses CLI arguments and environment variables into Config.
func Parse() (*Config, error) {
	var args Args
	p, err := arg.NewParser(arg.Config{}, &args)
	if err != nil {
		return nil, fmt.Errorf("arg parser: %w", err)
	}

	if err := p.Parse(os.Args[1:]); err != nil {
		if err == arg.ErrHelp {
			p.WriteHelp(os.Stdout)
			os.Exit(0)
		}
		if err == arg.ErrVersion {
			p.WriteUsage(os.Stdout)
			os.Exit(0)
		}
		return nil, err
	}

	if args.ShowVersion {
		fmt.Printf("ssh-mcp %s\n", Version)
		os.Exit(0)
	}

	return buildConfig(args), nil
}

func buildConfig(args Args) *Config {
	homeDir, _ := os.UserHomeDir()
	sshDir := filepath.Join(homeDir, ".ssh")

	knownHosts := args.KnownHosts
	if knownHosts == "" {
		knownHosts = filepath.Join(sshDir, "known_hosts")
	}

	sshConfigPath := args.SSHConfigPath
	if sshConfigPath == "" {
		sshConfigPath = filepath.Join(sshDir, "config")
	}

	return &Config{
		SSH: SSHConfig{
			KnownHostsPath:    knownHosts,
			VerifyHostKey:     !args.NoVerifyHost,
			ConfigPath:        sshConfigPath,
			KeySearchPaths:    defaultKeyPaths(sshDir),
			CommandTimeout:    args.CommandTimeout,
			ConnectionTimeout: 30 * time.Second,
			MaxIdleTime:       5 * time.Minute,
			AllowSudo:         args.EnableSudo,
			StripANSI:         true,
			MaxConnections:    args.MaxConnections,
		},
		Security: SecurityConfig{
			HostAllowlist:    []string(args.HostAllowlist),
			HostDenylist:     []string(args.HostDenylist),
			CommandAllowlist: []string(args.CommandAllowlist),
			CommandDenylist:  []string(args.CommandDenylist),
			RateLimit:        args.RateLimit,
			RateLimitFileOps: args.RateLimitFileOps,
			LocalBaseDir:     args.LocalBaseDir,
			MaxFileSize:      args.MaxFileSize,
		},
		Transport: TransportConfig{
			StdioEnabled: !args.DisableStdio,
			HTTPEnabled:  args.EnableHTTP,
			HTTPPort:     args.HTTPPort,
			HTTPPath:     "/mcp",
			HTTPHost:     "localhost", // hardcoded, not configurable
			HTTPToken:    args.HTTPToken,
		},
		DisabledTools: []string(args.DisableTools),
	}
}

func defaultKeyPaths(sshDir string) []string {
	return []string{
		filepath.Join(sshDir, "id_rsa"),
		filepath.Join(sshDir, "id_ed25519"),
		filepath.Join(sshDir, "id_ecdsa"),
		filepath.Join(sshDir, "id_dsa"),
	}
}
