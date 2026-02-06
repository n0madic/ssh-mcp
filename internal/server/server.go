package server

import (
	"context"
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
)

// Server is the SSH MCP server.
type Server struct {
	mcpServer   *mcp.Server
	pool        *connection.Pool
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

	s := &Server{
		mcpServer:   mcpServer,
		pool:        pool,
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
	}
	disconnectDeps := &tools.DisconnectDeps{Pool: s.pool}
	sessionsDeps := &tools.SessionsDeps{Pool: s.pool}
	fileUploadDeps := &tools.FileUploadDeps{
		Pool: s.pool, LocalBaseDir: s.cfg.Security.LocalBaseDir, RateLimiter: fileRateLimiter,
	}
	fileDownloadDeps := &tools.FileDownloadDeps{
		Pool: s.pool, LocalBaseDir: s.cfg.Security.LocalBaseDir, RateLimiter: fileRateLimiter,
	}
	fileEditDeps := &tools.FileEditDeps{
		Pool: s.pool, RateLimiter: fileRateLimiter, MaxFileSize: s.cfg.Security.MaxFileSize,
	}
	dirListDeps := &tools.DirListDeps{Pool: s.pool, RateLimiter: fileRateLimiter}
	dirUploadDeps := &tools.DirUploadDeps{
		Pool: s.pool, LocalBaseDir: s.cfg.Security.LocalBaseDir, RateLimiter: fileRateLimiter,
	}
	dirDownloadDeps := &tools.DirDownloadDeps{
		Pool: s.pool, LocalBaseDir: s.cfg.Security.LocalBaseDir, RateLimiter: fileRateLimiter,
	}
	fileStatDeps := &tools.FileStatDeps{Pool: s.pool, RateLimiter: fileRateLimiter}
	fileRenameDeps := &tools.FileRenameDeps{Pool: s.pool, RateLimiter: fileRateLimiter}

	// ssh_connect
	if !s.isToolDisabled("ssh_connect") {
		mcp.AddTool(s.mcpServer, &mcp.Tool{
			Name:        "ssh_connect",
			Description: "Connect to a remote host via SSH. Only 'host' is required — authentication is automatic (tries SSH keys from ~/.ssh/, ssh-agent, then ~/.ssh/config). Do NOT ask the user for auth details unless connection fails. Returns a session_id for use with other tools.",
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

	// ssh_upload_file
	if !s.isToolDisabled("ssh_upload_file") {
		mcp.AddTool(s.mcpServer, &mcp.Tool{
			Name:        "ssh_upload_file",
			Description: "Upload a local file to a remote host via SFTP. Preserves file permissions.",
			Annotations: &mcp.ToolAnnotations{
				Title:           "SSH Upload File",
				ReadOnlyHint:    false,
				DestructiveHint: boolPtr(false),
				IdempotentHint:  false,
				OpenWorldHint:   boolPtr(true),
			},
		}, func(ctx context.Context, _ *mcp.CallToolRequest, input tools.SSHUploadFileInput) (*mcp.CallToolResult, any, error) {
			out, err := tools.HandleUploadFile(ctx, fileUploadDeps, input)
			if err != nil {
				return nil, nil, err
			}
			return textResult(out.Text()), nil, nil
		})
	}

	// ssh_download_file
	if !s.isToolDisabled("ssh_download_file") {
		mcp.AddTool(s.mcpServer, &mcp.Tool{
			Name:        "ssh_download_file",
			Description: "Download a file from a remote host via SFTP. Preserves file permissions.",
			Annotations: &mcp.ToolAnnotations{
				Title:           "SSH Download File",
				ReadOnlyHint:    true,
				DestructiveHint: boolPtr(false),
				IdempotentHint:  true,
				OpenWorldHint:   boolPtr(true),
			},
		}, func(ctx context.Context, _ *mcp.CallToolRequest, input tools.SSHDownloadFileInput) (*mcp.CallToolResult, any, error) {
			out, err := tools.HandleDownloadFile(ctx, fileDownloadDeps, input)
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
			Description: "Edit a file on a remote host. Supports 'replace' mode (full content replacement) and 'patch' mode (find and replace a string). Creates .bak backup by default.",
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

	// ssh_list_directory
	if !s.isToolDisabled("ssh_list_directory") {
		mcp.AddTool(s.mcpServer, &mcp.Tool{
			Name:        "ssh_list_directory",
			Description: "List the contents of a remote directory via SFTP. Returns file names, sizes, permissions, and modification times.",
			Annotations: &mcp.ToolAnnotations{
				Title:           "SSH List Directory",
				ReadOnlyHint:    true,
				DestructiveHint: boolPtr(false),
				IdempotentHint:  true,
				OpenWorldHint:   boolPtr(true),
			},
		}, func(ctx context.Context, _ *mcp.CallToolRequest, input tools.SSHListDirectoryInput) (*mcp.CallToolResult, any, error) {
			out, err := tools.HandleListDirectory(ctx, dirListDeps, input)
			if err != nil {
				return nil, nil, err
			}
			return textResult(out.Text()), nil, nil
		})
	}

	// ssh_upload_directory
	if !s.isToolDisabled("ssh_upload_directory") {
		mcp.AddTool(s.mcpServer, &mcp.Tool{
			Name:        "ssh_upload_directory",
			Description: "Recursively upload a local directory to a remote host via SFTP. Preserves directory structure and file permissions.",
			Annotations: &mcp.ToolAnnotations{
				Title:           "SSH Upload Directory",
				ReadOnlyHint:    false,
				DestructiveHint: boolPtr(false),
				IdempotentHint:  false,
				OpenWorldHint:   boolPtr(true),
			},
		}, func(ctx context.Context, _ *mcp.CallToolRequest, input tools.SSHUploadDirectoryInput) (*mcp.CallToolResult, any, error) {
			out, err := tools.HandleUploadDirectory(ctx, dirUploadDeps, input)
			if err != nil {
				return nil, nil, err
			}
			return textResult(out.Text()), nil, nil
		})
	}

	// ssh_download_directory
	if !s.isToolDisabled("ssh_download_directory") {
		mcp.AddTool(s.mcpServer, &mcp.Tool{
			Name:        "ssh_download_directory",
			Description: "Recursively download a remote directory to a local path via SFTP. Preserves directory structure and file permissions.",
			Annotations: &mcp.ToolAnnotations{
				Title:           "SSH Download Directory",
				ReadOnlyHint:    true,
				DestructiveHint: boolPtr(false),
				IdempotentHint:  true,
				OpenWorldHint:   boolPtr(true),
			},
		}, func(ctx context.Context, _ *mcp.CallToolRequest, input tools.SSHDownloadDirectoryInput) (*mcp.CallToolResult, any, error) {
			out, err := tools.HandleDownloadDirectory(ctx, dirDownloadDeps, input)
			if err != nil {
				return nil, nil, err
			}
			return textResult(out.Text()), nil, nil
		})
	}

	// ssh_file_stat
	if !s.isToolDisabled("ssh_file_stat") {
		mcp.AddTool(s.mcpServer, &mcp.Tool{
			Name:        "ssh_file_stat",
			Description: "Get file or directory information (size, permissions, modification time). Supports ~ for remote home directory.",
			Annotations: &mcp.ToolAnnotations{
				Title:           "SSH File Stat",
				ReadOnlyHint:    true,
				DestructiveHint: boolPtr(false),
				IdempotentHint:  true,
				OpenWorldHint:   boolPtr(true),
			},
		}, func(ctx context.Context, _ *mcp.CallToolRequest, input tools.SSHFileStatInput) (*mcp.CallToolResult, any, error) {
			out, err := tools.HandleFileStat(ctx, fileStatDeps, input)
			if err != nil {
				return nil, nil, err
			}
			return textResult(out.Text()), nil, nil
		})
	}

	// ssh_rename
	if !s.isToolDisabled("ssh_rename") {
		mcp.AddTool(s.mcpServer, &mcp.Tool{
			Name:        "ssh_rename",
			Description: "Rename or move a file/directory on remote host. Supports ~ for paths.",
			Annotations: &mcp.ToolAnnotations{
				Title:           "SSH Rename",
				ReadOnlyHint:    false,
				DestructiveHint: boolPtr(false),
				IdempotentHint:  false,
				OpenWorldHint:   boolPtr(true),
			},
		}, func(ctx context.Context, _ *mcp.CallToolRequest, input tools.SSHRenameInput) (*mcp.CallToolResult, any, error) {
			out, err := tools.HandleRename(ctx, fileRenameDeps, input)
			if err != nil {
				return nil, nil, err
			}
			return textResult(out.Text()), nil, nil
		})
	}
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

		if authHeader[len(prefix):] != token {
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
		Addr:    addr,
		Handler: httpHandler,
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
	log.Println("Closing all SSH connections...")
	s.pool.CloseAll()
	log.Println("Shutdown complete")
}
