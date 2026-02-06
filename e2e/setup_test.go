package e2e

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/n0madic/ssh-mcp/internal/config"
	"github.com/n0madic/ssh-mcp/internal/server"
)

// sharedEnv is the singleton test environment shared across all E2E tests.
var sharedEnv *mcpTestEnv

// sharedCancel cancels the shared context.
var sharedCancel context.CancelFunc

// sshContainer holds the SSH container details.
type sshContainer struct {
	container testcontainers.Container
	host      string
	port      int
}

// mcpTestEnv holds the MCP server and client session for testing.
type mcpTestEnv struct {
	session *mcp.ClientSession
	sshHost string
	sshPort int
}

// setupSharedEnv creates the shared environment once for all E2E tests.
func setupSharedEnv(t *testing.T) *mcpTestEnv {
	t.Helper()

	if sharedEnv != nil {
		return sharedEnv
	}

	ctx, cancel := context.WithCancel(context.Background())
	sharedCancel = cancel

	ssh := startSSHContainer(ctx, t)
	sharedEnv = startMCPServer(ctx, t, ssh)
	return sharedEnv
}

// startSSHContainer creates and starts a Docker SSH server container.
func startSSHContainer(ctx context.Context, t *testing.T) *sshContainer {
	t.Helper()

	_, currentFile, _, _ := runtime.Caller(0)
	e2eDir := filepath.Dir(currentFile)

	ctr, err := testcontainers.Run(ctx, "",
		testcontainers.WithDockerfile(testcontainers.FromDockerfile{
			Context:    e2eDir,
			Dockerfile: "Dockerfile",
			KeepImage:  false,
		}),
		testcontainers.WithExposedPorts("22/tcp"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("22/tcp").
				WithStartupTimeout(60*time.Second),
		),
	)
	testcontainers.CleanupContainer(t, ctr)
	if err != nil {
		t.Fatalf("failed to start SSH container: %v", err)
	}

	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %v", err)
	}

	mappedPort, err := ctr.MappedPort(ctx, "22/tcp")
	if err != nil {
		t.Fatalf("failed to get mapped port: %v", err)
	}

	return &sshContainer{
		container: ctr,
		host:      host,
		port:      mappedPort.Int(),
	}
}

// startMCPServer starts the MCP server with HTTP transport and returns a connected client session.
func startMCPServer(ctx context.Context, t *testing.T, ssh *sshContainer) *mcpTestEnv {
	t.Helper()

	// Find a free port for the HTTP server.
	httpPort := getFreePort(t)

	cfg := &config.Config{
		SSH: config.SSHConfig{
			KnownHostsPath:    "/dev/null",
			VerifyHostKey:     false,
			ConfigPath:        "/dev/null",
			KeySearchPaths:    []string{},
			CommandTimeout:    30 * time.Second,
			ConnectionTimeout: 30 * time.Second,
			MaxIdleTime:       5 * time.Minute,
			AllowSudo:         true,
			StripANSI:         true,
		},
		Security: config.SecurityConfig{
			RateLimit: 600,
		},
		Transport: config.TransportConfig{
			StdioEnabled: false,
			HTTPEnabled:  true,
			HTTPPort:     httpPort,
			HTTPPath:     "/mcp",
			HTTPHost:     "localhost",
		},
	}

	srv, err := server.New(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to create MCP server: %v", err)
	}

	// Run server in background.
	srvCtx, srvCancel := context.WithCancel(ctx)
	srvDone := make(chan struct{})
	go func() {
		defer close(srvDone)
		srv.Run(srvCtx)
	}()

	t.Cleanup(func() {
		srvCancel()
		<-srvDone
	})

	// Wait for HTTP server to be ready.
	waitForHTTP(t, httpPort)
	endpoint := fmt.Sprintf("http://localhost:%d/mcp", httpPort)

	// Create MCP client and connect.
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "e2e-test-client",
		Version: "1.0.0",
	}, nil)

	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: endpoint,
	}, nil)
	if err != nil {
		t.Fatalf("failed to connect MCP client: %v", err)
	}

	t.Cleanup(func() {
		session.Close()
	})

	return &mcpTestEnv{
		session: session,
		sshHost: ssh.host,
		sshPort: ssh.port,
	}
}

// callTool calls an MCP tool and returns the text result.
func callTool(t *testing.T, env *mcpTestEnv, toolName string, args map[string]any) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := env.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool %q failed: %v", toolName, err)
	}

	if result.IsError {
		for _, content := range result.Content {
			if tc, ok := content.(*mcp.TextContent); ok {
				t.Fatalf("tool %q returned error: %s", toolName, tc.Text)
			}
		}
		t.Fatalf("tool %q returned error with no text content", toolName)
	}

	var text string
	for _, content := range result.Content {
		if tc, ok := content.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	return text
}

// sshConnect connects to the SSH container via the MCP server and returns the session ID.
func sshConnect(t *testing.T, env *mcpTestEnv) string {
	t.Helper()

	hostStr := fmt.Sprintf("testuser:password@%s:%d", env.sshHost, env.sshPort)
	text := callTool(t, env, "ssh_connect", map[string]any{
		"host": hostStr,
	})

	sessionID := fmt.Sprintf("testuser@%s:%d", env.sshHost, env.sshPort)
	if text == "" {
		t.Fatal("ssh_connect returned empty text")
	}
	t.Logf("ssh_connect response: %s", text)
	return sessionID
}

// getFreePort returns an available TCP port.
func getFreePort(t *testing.T) int {
	t.Helper()

	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	defer l.Close()

	_, portStr, _ := net.SplitHostPort(l.Addr().String())
	port, _ := strconv.Atoi(portStr)
	return port
}

// waitForHTTP polls the TCP address until the HTTP server is accepting connections.
func waitForHTTP(t *testing.T, httpPort int) {
	t.Helper()

	addr := fmt.Sprintf("localhost:%d", httpPort)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("HTTP server at %s did not become ready", addr)
}
