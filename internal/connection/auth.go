package connection

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kevinburke/ssh_config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/n0madic/ssh-mcp/internal/config"
)

// ConnectParams holds parameters for establishing an SSH connection.
type ConnectParams struct {
	Host         string
	Port         int
	User         string
	Password     string
	KeyPath      string
	UseSSHConfig bool
}

// ResolvedHost holds resolved SSH connection details from ssh_config.
type ResolvedHost struct {
	HostName     string
	Port         int
	User         string
	IdentityFile string
}

// AuthDiscovery handles SSH authentication method discovery.
type AuthDiscovery struct {
	cfg *config.SSHConfig
}

// NewAuthDiscovery creates a new AuthDiscovery.
func NewAuthDiscovery(cfg *config.SSHConfig) *AuthDiscovery {
	return &AuthDiscovery{cfg: cfg}
}

// ResolveHost resolves an SSH alias from ssh_config to actual connection details.
func (a *AuthDiscovery) ResolveHost(alias string) *ResolvedHost {
	resolved := &ResolvedHost{
		HostName: alias,
		Port:     22,
	}

	f, err := os.Open(a.cfg.ConfigPath)
	if err != nil {
		return resolved
	}
	defer f.Close()

	sshCfg, err := ssh_config.Decode(f)
	if err != nil {
		return resolved
	}

	if hostname, err := sshCfg.Get(alias, "HostName"); err == nil && hostname != "" {
		resolved.HostName = hostname
	}
	if portStr, err := sshCfg.Get(alias, "Port"); err == nil && portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			resolved.Port = port
		}
	}
	if user, err := sshCfg.Get(alias, "User"); err == nil && user != "" {
		resolved.User = user
	}
	if identityFile, err := sshCfg.Get(alias, "IdentityFile"); err == nil && identityFile != "" {
		resolved.IdentityFile = expandPath(identityFile)
	}

	return resolved
}

// BuildAuthMethods constructs SSH authentication methods from the given parameters.
// Keys are tried first, then password.
func (a *AuthDiscovery) BuildAuthMethods(params ConnectParams) []ssh.AuthMethod {
	var methods []ssh.AuthMethod

	// Try explicit key path first.
	if params.KeyPath != "" {
		if method := a.loadKeyAuth(expandPath(params.KeyPath)); method != nil {
			methods = append(methods, method)
		}
	}

	// Try default key paths.
	for _, keyPath := range a.cfg.KeySearchPaths {
		if method := a.loadKeyAuth(keyPath); method != nil {
			methods = append(methods, method)
		}
	}

	// Try password auth last.
	if params.Password != "" {
		methods = append(methods, ssh.Password(params.Password))
	}

	return methods
}

// BuildClientConfig creates an ssh.ClientConfig from the given parameters.
func (a *AuthDiscovery) BuildClientConfig(params ConnectParams) (*ssh.ClientConfig, error) {
	authMethods := a.BuildAuthMethods(params)
	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication methods available")
	}

	hostKeyCallback, err := a.buildHostKeyCallback()
	if err != nil {
		return nil, fmt.Errorf("host key callback: %w", err)
	}

	return &ssh.ClientConfig{
		User:            params.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         a.cfg.ConnectionTimeout,
	}, nil
}

// ParseHostString parses "user:password@host:port" format into ConnectParams.
func ParseHostString(s string) ConnectParams {
	params := ConnectParams{Port: 22}

	// Extract user:password@ prefix.
	if idx := strings.LastIndex(s, "@"); idx != -1 {
		userPart := s[:idx]
		s = s[idx+1:]
		if colonIdx := strings.Index(userPart, ":"); colonIdx != -1 {
			params.User = userPart[:colonIdx]
			params.Password = userPart[colonIdx+1:]
		} else {
			params.User = userPart
		}
	}

	// Extract host:port.
	host, portStr, err := net.SplitHostPort(s)
	if err != nil {
		// No port specified.
		params.Host = s
	} else {
		params.Host = host
		if port, err := strconv.Atoi(portStr); err == nil {
			params.Port = port
		}
	}

	return params
}

func (a *AuthDiscovery) loadKeyAuth(keyPath string) ssh.AuthMethod {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil
	}

	return ssh.PublicKeys(signer)
}

func (a *AuthDiscovery) buildHostKeyCallback() (ssh.HostKeyCallback, error) {
	if !a.cfg.VerifyHostKey {
		return ssh.InsecureIgnoreHostKey(), nil
	}

	if _, err := os.Stat(a.cfg.KnownHostsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("host key verification is enabled but known_hosts file %q does not exist; "+
			"use --no-verify-host-key to disable verification or create the file with ssh-keyscan", a.cfg.KnownHostsPath)
	}

	callback, err := knownhosts.New(a.cfg.KnownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse known_hosts %s: %w", a.cfg.KnownHostsPath, err)
	}

	return callback, nil
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
