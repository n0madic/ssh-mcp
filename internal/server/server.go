package server

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/n0madic/ssh-mcp/internal/config"
	"github.com/n0madic/ssh-mcp/internal/connection"
	"github.com/n0madic/ssh-mcp/internal/security"
	"github.com/n0madic/ssh-mcp/internal/tools"
	"github.com/n0madic/ssh-mcp/internal/tunnel"
)

// Server is the SSH MCP server.
type Server struct {
	mcpServer   *mcp.Server
	pool        *connection.Pool
	termPool    *connection.TerminalPool
	tunnelPool  *tunnel.TunnelPool
	auth        *connection.AuthDiscovery
	filter      *security.Filter
	rateLimiter *security.RateLimiter
	cfg         *config.Config
}

func boolPtr(b bool) *bool {
	return &b
}

// textResult creates a CallToolResult with a single text content.
func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
	}
}

// isToolDisabled checks if a tool is in the disabled list.
func (s *Server) isToolDisabled(toolName string) bool {
	return slices.Contains(s.cfg.DisabledTools, toolName)
}

// New creates and configures a new SSH MCP server.
func New(ctx context.Context, cfg *config.Config) (*Server, error) {
	auth := connection.NewAuthDiscovery(&cfg.SSH)
	pool := connection.NewPool(&cfg.SSH, auth)

	filter, err := security.NewFilter(
		cfg.Security.HostAllowlist,
		cfg.Security.HostDenylist,
		cfg.Security.CommandAllowlist,
		cfg.Security.CommandDenylist,
	)
	if err != nil {
		return nil, fmt.Errorf("create filter: %w", err)
	}

	rateLimiter := security.NewRateLimiter(cfg.Security.RateLimit)

	mcpServer := mcp.NewServer(
		&mcp.Implementation{
			Name:    "ssh-mcp",
			Version: config.Version,
		},
		nil,
	)

	var tunnelPool *tunnel.TunnelPool
	if cfg.SSH.AllowTunnels {
		tunnelPool = tunnel.NewTunnelPool(cfg.SSH.MaxTunnels)
	}

	s := &Server{
		mcpServer:   mcpServer,
		pool:        pool,
		termPool:    connection.NewTerminalPool(cfg.SSH.MaxTerminals),
		tunnelPool:  tunnelPool,
		auth:        auth,
		filter:      filter,
		rateLimiter: rateLimiter,
		cfg:         cfg,
	}

	s.registerTools()
	pool.StartIdleCleanup(ctx)
	rateLimiter.StartCleanup(ctx, 10*time.Minute, 30*time.Minute)

	return s, nil
}

// fileOpsRateLimiter returns the rate limiter if file ops rate limiting is enabled, nil otherwise.
func (s *Server) fileOpsRateLimiter() *security.RateLimiter {
	if s.cfg.Security.RateLimitFileOps {
		return s.rateLimiter
	}
	return nil
}

func (s *Server) registerTools() {
	fileRateLimiter := s.fileOpsRateLimiter()

	connectDeps := &tools.ConnectDeps{
		Pool: s.pool, Auth: s.auth, Filter: s.filter, RateLimiter: s.rateLimiter,
	}
	executeDeps := &tools.ExecuteDeps{
		Pool: s.pool, Filter: s.filter, RateLimiter: s.rateLimiter, Config: &s.cfg.SSH,
		MaxOutputSize: s.cfg.SSH.MaxOutputSize,
	}
	disconnectDeps := &tools.DisconnectDeps{Pool: s.pool, TermPool: s.termPool, TunnelPool: s.tunnelPool}
	sessionsDeps := &tools.SessionsDeps{Pool: s.pool, TermPool: s.termPool, TunnelPool: s.tunnelPool}
	uploadDeps := &tools.UploadDeps{
		Pool: s.pool, LocalBaseDir: s.cfg.Security.LocalBaseDir, RateLimiter: fileRateLimiter,
	}
	downloadDeps := &tools.DownloadDeps{
		Pool: s.pool, LocalBaseDir: s.cfg.Security.LocalBaseDir, RateLimiter: fileRateLimiter,
	}
	fileEditDeps := &tools.FileEditDeps{
		Pool: s.pool, RateLimiter: fileRateLimiter, MaxFileSize: s.cfg.Security.MaxFileSize,
	}
	fileReadDeps := &tools.FileReadDeps{
		Pool: s.pool, RateLimiter: fileRateLimiter, MaxFileSize: s.cfg.Security.MaxFileSize,
	}

	// ssh_connect
	if !s.isToolDisabled("ssh_connect") {
		mcp.AddTool(s.mcpServer, &mcp.Tool{
			Name:        "ssh_connect",
			Description: "Connect to a remote host via SSH. Only 'host' is required — authentication is automatic (tries SSH keys from ~/.ssh/, ssh-agent, then ~/.ssh/config). SSH config aliases (~/.ssh/config) are resolved automatically. Do NOT ask the user for auth details unless connection fails. Returns a session_id for use with other tools.",
			Annotations: &mcp.ToolAnnotations{
				Title:           "SSH Connect",
				ReadOnlyHint:    false,
				DestructiveHint: boolPtr(false),
				IdempotentHint:  true,
				OpenWorldHint:   boolPtr(true),
			},
		}, func(ctx context.Context, _ *mcp.CallToolRequest, input tools.SSHConnectInput) (*mcp.CallToolResult, any, error) {
			out, err := tools.HandleConnect(ctx, connectDeps, input)
			if err != nil {
				return nil, nil, err
			}
			return textResult(out.Text()), nil, nil
		})
	}

	// ssh_execute
	if !s.isToolDisabled("ssh_execute") {
		mcp.AddTool(s.mcpServer, &mcp.Tool{
			Name:        "ssh_execute",
			Description: "Execute a command on a remote host via SSH. Supports sudo, working directory, and timeout. Returns stdout, stderr, exit code, and duration.",
			Annotations: &mcp.ToolAnnotations{
				Title:           "SSH Execute",
				ReadOnlyHint:    false,
				DestructiveHint: boolPtr(true),
				IdempotentHint:  false,
				OpenWorldHint:   boolPtr(true),
			},
		}, func(ctx context.Context, _ *mcp.CallToolRequest, input tools.SSHExecuteInput) (*mcp.CallToolResult, any, error) {
			out, err := tools.HandleExecute(ctx, executeDeps, input)
			if err != nil {
				return nil, nil, err
			}
			return textResult(out.Text()), nil, nil
		})
	}

	// ssh_disconnect
	if !s.isToolDisabled("ssh_disconnect") {
		mcp.AddTool(s.mcpServer, &mcp.Tool{
			Name:        "ssh_disconnect",
			Description: "Disconnect an active SSH session. The session_id will no longer be usable.",
			Annotations: &mcp.ToolAnnotations{
				Title:           "SSH Disconnect",
				ReadOnlyHint:    false,
				DestructiveHint: boolPtr(false),
				IdempotentHint:  true,
				OpenWorldHint:   boolPtr(false),
			},
		}, func(ctx context.Context, _ *mcp.CallToolRequest, input tools.SSHDisconnectInput) (*mcp.CallToolResult, any, error) {
			out, err := tools.HandleDisconnect(ctx, disconnectDeps, input)
			if err != nil {
				return nil, nil, err
			}
			return textResult(out.Text()), nil, nil
		})
	}

	// ssh_list_sessions
	if !s.isToolDisabled("ssh_list_sessions") {
		mcp.AddTool(s.mcpServer, &mcp.Tool{
			Name:        "ssh_list_sessions",
			Description: "List all active SSH sessions with their connection details and statistics.",
			Annotations: &mcp.ToolAnnotations{
				Title:           "SSH List Sessions",
				ReadOnlyHint:    true,
				DestructiveHint: boolPtr(false),
				IdempotentHint:  true,
				OpenWorldHint:   boolPtr(false),
			},
		}, func(ctx context.Context, _ *mcp.CallToolRequest, input tools.SSHListSessionsInput) (*mcp.CallToolResult, any, error) {
			out, err := tools.HandleListSessions(ctx, sessionsDeps, input)
			if err != nil {
				return nil, nil, err
			}
			return textResult(out.Text()), nil, nil
		})
	}

	// ssh_upload
	if !s.isToolDisabled("ssh_upload") {
		mcp.AddTool(s.mcpServer, &mcp.Tool{
			Name:        "ssh_upload",
			Description: "Upload a local file or directory to a remote host via SFTP. Automatically detects whether the local path is a file or directory. Preserves file permissions and directory structure.",
			Annotations: &mcp.ToolAnnotations{
				Title:           "SSH Upload",
				ReadOnlyHint:    false,
				DestructiveHint: boolPtr(false),
				IdempotentHint:  false,
				OpenWorldHint:   boolPtr(true),
			},
		}, func(ctx context.Context, _ *mcp.CallToolRequest, input tools.SSHUploadInput) (*mcp.CallToolResult, any, error) {
			out, err := tools.HandleUpload(ctx, uploadDeps, input)
			if err != nil {
				return nil, nil, err
			}
			return textResult(out.Text()), nil, nil
		})
	}

	// ssh_download
	if !s.isToolDisabled("ssh_download") {
		mcp.AddTool(s.mcpServer, &mcp.Tool{
			Name:        "ssh_download",
			Description: "Download a file or directory from a remote host via SFTP. Automatically detects whether the remote path is a file or directory. Preserves file permissions and directory structure.",
			Annotations: &mcp.ToolAnnotations{
				Title:           "SSH Download",
				ReadOnlyHint:    true,
				DestructiveHint: boolPtr(false),
				IdempotentHint:  true,
				OpenWorldHint:   boolPtr(true),
			},
		}, func(ctx context.Context, _ *mcp.CallToolRequest, input tools.SSHDownloadInput) (*mcp.CallToolResult, any, error) {
			out, err := tools.HandleDownload(ctx, downloadDeps, input)
			if err != nil {
				return nil, nil, err
			}
			return textResult(out.Text()), nil, nil
		})
	}

	// ssh_edit_file
	if !s.isToolDisabled("ssh_edit_file") {
		mcp.AddTool(s.mcpServer, &mcp.Tool{
			Name:        "ssh_edit_file",
			Description: "Edit a file on a remote host. Supports 'replace' mode (full content replacement or new file creation) and 'patch' mode (find and replace a string). Creates .bak backup by default.",
			Annotations: &mcp.ToolAnnotations{
				Title:           "SSH Edit File",
				ReadOnlyHint:    false,
				DestructiveHint: boolPtr(true),
				IdempotentHint:  false,
				OpenWorldHint:   boolPtr(true),
			},
		}, func(ctx context.Context, _ *mcp.CallToolRequest, input tools.SSHEditFileInput) (*mcp.CallToolResult, any, error) {
			out, err := tools.HandleEditFile(ctx, fileEditDeps, input)
			if err != nil {
				return nil, nil, err
			}
			return textResult(out.Text()), nil, nil
		})
	}

	// ssh_read_file
	if !s.isToolDisabled("ssh_read_file") {
		mcp.AddTool(s.mcpServer, &mcp.Tool{
			Name:        "ssh_read_file",
			Description: "Read a file from a remote host with optional line offset and limit. Returns content with line numbers. Supports ~ for home directory.",
			Annotations: &mcp.ToolAnnotations{
				Title:           "SSH Read File",
				ReadOnlyHint:    true,
				DestructiveHint: boolPtr(false),
				IdempotentHint:  true,
				OpenWorldHint:   boolPtr(true),
			},
		}, func(ctx context.Context, _ *mcp.CallToolRequest, input tools.SSHReadFileInput) (*mcp.CallToolResult, any, error) {
			out, err := tools.HandleReadFile(ctx, fileReadDeps, input)
			if err != nil {
				return nil, nil, err
			}
			return textResult(out.Text()), nil, nil
		})
	}

	if s.cfg.SSH.AllowTerminal {
		terminalDeps := &tools.TerminalDeps{
			Pool:          s.pool,
			TermPool:      s.termPool,
			RateLimiter:   s.rateLimiter,
			MaxOutputSize: s.cfg.SSH.MaxOutputSize,
		}

		// ssh_open_terminal
		if !s.isToolDisabled("ssh_open_terminal") {
			mcp.AddTool(s.mcpServer, &mcp.Tool{
				Name:        "ssh_open_terminal",
				Description: "Open an interactive PTY terminal session over SSH. Returns a terminal_id for use with ssh_send_input, ssh_read_output, and ssh_close_terminal.",
				Annotations: &mcp.ToolAnnotations{
					Title:           "SSH Open Terminal",
					ReadOnlyHint:    false,
					DestructiveHint: boolPtr(false),
					IdempotentHint:  false,
					OpenWorldHint:   boolPtr(true),
				},
			}, func(ctx context.Context, _ *mcp.CallToolRequest, input tools.SSHOpenTerminalInput) (*mcp.CallToolResult, any, error) {
				out, err := tools.HandleOpenTerminal(ctx, terminalDeps, input)
				if err != nil {
					return nil, nil, err
				}
				return textResult(out.Text()), nil, nil
			})
		}

		// ssh_send_input
		if !s.isToolDisabled("ssh_send_input") {
			mcp.AddTool(s.mcpServer, &mcp.Tool{
				Name:        "ssh_send_input",
				Description: "Send text or a special key (CTRL_C, ENTER, TAB, etc.) to an interactive PTY terminal and read back the new output. Always returns output captured during wait_ms — no need to call ssh_read_output afterwards for quick commands. Use ssh_read_output only for long-running commands or TUI programs that produce output without further input.",
				Annotations: &mcp.ToolAnnotations{
					Title:           "SSH Send Input",
					ReadOnlyHint:    false,
					DestructiveHint: boolPtr(true),
					IdempotentHint:  false,
					OpenWorldHint:   boolPtr(true),
				},
			}, func(ctx context.Context, _ *mcp.CallToolRequest, input tools.SSHSendInputInput) (*mcp.CallToolResult, any, error) {
				out, err := tools.HandleSendInput(ctx, terminalDeps, input)
				if err != nil {
					return nil, nil, err
				}
				return textResult(out.Text()), nil, nil
			})
		}

		// ssh_read_output
		if !s.isToolDisabled("ssh_read_output") {
			mcp.AddTool(s.mcpServer, &mcp.Tool{
				Name:        "ssh_read_output",
				Description: "Read buffered output from a PTY terminal since the last read. Optionally waits up to wait_ms milliseconds for new data. Use this for long-running commands or TUI programs that produce output independently of input; for quick commands prefer ssh_send_input which already returns output.",
				Annotations: &mcp.ToolAnnotations{
					Title:           "SSH Read Output",
					ReadOnlyHint:    true,
					DestructiveHint: boolPtr(false),
					IdempotentHint:  false,
					OpenWorldHint:   boolPtr(false),
				},
			}, func(ctx context.Context, _ *mcp.CallToolRequest, input tools.SSHReadOutputInput) (*mcp.CallToolResult, any, error) {
				out, err := tools.HandleReadOutput(ctx, terminalDeps, input)
				if err != nil {
					return nil, nil, err
				}
				return textResult(out.Text()), nil, nil
			})
		}

		// ssh_close_terminal
		if !s.isToolDisabled("ssh_close_terminal") {
			mcp.AddTool(s.mcpServer, &mcp.Tool{
				Name:        "ssh_close_terminal",
				Description: "Close an active PTY terminal session. The terminal_id will no longer be usable.",
				Annotations: &mcp.ToolAnnotations{
					Title:           "SSH Close Terminal",
					ReadOnlyHint:    false,
					DestructiveHint: boolPtr(false),
					IdempotentHint:  true,
					OpenWorldHint:   boolPtr(false),
				},
			}, func(ctx context.Context, _ *mcp.CallToolRequest, input tools.SSHCloseTerminalInput) (*mcp.CallToolResult, any, error) {
				out, err := tools.HandleCloseTerminal(ctx, terminalDeps, input)
				if err != nil {
					return nil, nil, err
				}
				return textResult(out.Text()), nil, nil
			})
		}
	} // AllowTerminal

	if s.cfg.SSH.AllowTunnels {
		tunnelDeps := &tools.TunnelDeps{
			Pool:        s.pool,
			TunnelPool:  s.tunnelPool,
			RateLimiter: s.rateLimiter,
		}

		// ssh_tunnel_create
		if !s.isToolDisabled("ssh_tunnel_create") {
			mcp.AddTool(s.mcpServer, &mcp.Tool{
				Name:        "ssh_tunnel_create",
				Description: "Create a local port forwarding tunnel (localhost:port → remote:port via SSH). Binds a local port and forwards connections through the SSH session to the specified remote address. Returns the tunnel_id and local address for use.",
				Annotations: &mcp.ToolAnnotations{
					Title:           "SSH Tunnel Create",
					ReadOnlyHint:    false,
					DestructiveHint: boolPtr(false),
					IdempotentHint:  false,
					OpenWorldHint:   boolPtr(true),
				},
			}, func(ctx context.Context, _ *mcp.CallToolRequest, input tools.SSHTunnelCreateInput) (*mcp.CallToolResult, any, error) {
				out, err := tools.HandleTunnelCreate(ctx, tunnelDeps, input)
				if err != nil {
					return nil, nil, err
				}
				return textResult(out.Text()), nil, nil
			})
		}

		// ssh_tunnel_list
		if !s.isToolDisabled("ssh_tunnel_list") {
			mcp.AddTool(s.mcpServer, &mcp.Tool{
				Name:        "ssh_tunnel_list",
				Description: "List all active SSH tunnels with their connection details. Optionally filter by session ID.",
				Annotations: &mcp.ToolAnnotations{
					Title:           "SSH Tunnel List",
					ReadOnlyHint:    true,
					DestructiveHint: boolPtr(false),
					IdempotentHint:  true,
					OpenWorldHint:   boolPtr(false),
				},
			}, func(ctx context.Context, _ *mcp.CallToolRequest, input tools.SSHTunnelListInput) (*mcp.CallToolResult, any, error) {
				out, err := tools.HandleTunnelList(ctx, tunnelDeps, input)
				if err != nil {
					return nil, nil, err
				}
				return textResult(out.Text()), nil, nil
			})
		}

		// ssh_tunnel_close
		if !s.isToolDisabled("ssh_tunnel_close") {
			mcp.AddTool(s.mcpServer, &mcp.Tool{
				Name:        "ssh_tunnel_close",
				Description: "Close an active SSH tunnel. The tunnel_id will no longer be usable.",
				Annotations: &mcp.ToolAnnotations{
					Title:           "SSH Tunnel Close",
					ReadOnlyHint:    false,
					DestructiveHint: boolPtr(false),
					IdempotentHint:  true,
					OpenWorldHint:   boolPtr(false),
				},
			}, func(ctx context.Context, _ *mcp.CallToolRequest, input tools.SSHTunnelCloseInput) (*mcp.CallToolResult, any, error) {
				out, err := tools.HandleTunnelClose(ctx, tunnelDeps, input)
				if err != nil {
					return nil, nil, err
				}
				return textResult(out.Text()), nil, nil
			})
		}
	} // AllowTunnels
}

// authMiddleware wraps an HTTP handler with bearer token authentication.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := s.cfg.Transport.HTTPToken
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "missing Authorization header", http.StatusUnauthorized)
			return
		}

		const prefix = "Bearer "
		if !strings.HasPrefix(authHeader, prefix) {
			http.Error(w, "invalid Authorization header format (expected Bearer token)", http.StatusUnauthorized)
			return
		}

		if subtle.ConstantTimeCompare([]byte(authHeader[len(prefix):]), []byte(token)) != 1 {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Run starts the MCP server with the configured transports.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 2)

	if s.cfg.Transport.HTTPEnabled {
		go func() {
			errCh <- s.runHTTP(ctx)
		}()
	}

	if s.cfg.Transport.StdioEnabled {
		if isStdinTerminal() {
			if s.cfg.Transport.HTTPEnabled {
				log.Println("Stdin is a terminal, skipping stdio transport (HTTP is active)")
			} else {
				return fmt.Errorf("stdin is a terminal; stdio transport expects an MCP client on stdin.\n" +
					"  Use with an MCP client (e.g. Claude Desktop) or start with --enable-http for HTTP transport")
			}
		} else {
			go func() {
				errCh <- s.runStdio(ctx)
			}()
		}
	}

	// Wait for context cancellation or first error.
	select {
	case <-ctx.Done():
		log.Println("Shutting down...")
	case err := <-errCh:
		if err != nil {
			log.Printf("Transport error: %v", err)
		}
	}

	s.shutdown()
	return nil
}

func (s *Server) runStdio(ctx context.Context) error {
	log.Println("Starting stdio transport...")
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}

// isStdinTerminal returns true if stdin is connected to a terminal (TTY).
func isStdinTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func (s *Server) runHTTP(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Transport.HTTPHost, s.cfg.Transport.HTTPPort)
	log.Printf("Starting HTTP transport on %s%s", addr, s.cfg.Transport.HTTPPath)

	handler := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server {
			return s.mcpServer
		},
		nil,
	)

	mux := http.NewServeMux()
	mux.Handle(s.cfg.Transport.HTTPPath, handler)

	// Wrap with auth middleware.
	var httpHandler http.Handler = mux
	httpHandler = s.authMiddleware(httpHandler)

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           httpHandler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		httpServer.Shutdown(shutdownCtx)
	}()

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server: %w", err)
	}
	return nil
}

func (s *Server) shutdown() {
	if s.tunnelPool != nil {
		log.Println("Closing all tunnels...")
		s.tunnelPool.CloseAll()
	}
	log.Println("Closing all terminal sessions...")
	s.termPool.CloseAll()
	log.Println("Closing all SSH connections...")
	s.pool.CloseAll()
	log.Println("Shutdown complete")
}
