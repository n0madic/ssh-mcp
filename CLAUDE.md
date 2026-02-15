# SSH MCP Server - Development Guide

## Quick Reference

```bash
go build .              # Build (creates ./ssh-mcp)
go test ./...           # Run tests
go vet ./...            # Lint
./ssh-mcp               # Run (stdio mode)
./ssh-mcp --enable-http # Run with HTTP
./ssh-mcp --help        # Show all CLI options
```

## Architecture

SSH MCP Server provides 12 tools to AI agents via the Model Context Protocol:

- **Core**: `ssh_connect`, `ssh_execute`, `ssh_disconnect`, `ssh_list_sessions`
- **Files**: `ssh_upload_file`, `ssh_download_file`, `ssh_edit_file`, `ssh_file_stat`, `ssh_rename`
- **Directories**: `ssh_list_directory`, `ssh_upload_directory`, `ssh_download_directory`

### Key Design Decisions

- **SessionID = `user@host:port`** — reconnecting to the same host reuses the connection
- **Auto-reconnect** — transparent reconnection when a connection drops
- **SFTP per-operation** — SFTP clients are created and closed per-operation to avoid holding channels
- **Security pipeline** — every handler: rate limit → host/command filter → path check → local path validation → execute
- **HTTP localhost only** — hardcoded, not configurable
- **HTTP bearer auth** — optional `--http-token` for HTTP transport authentication
- **Local path restriction** — `--local-base-dir` restricts upload/download local paths
- **No credential persistence** — passwords are not stored in the connection pool; only `ssh.ClientConfig` is retained for auto-reconnect
- **Auto-anchored filters** — regex patterns are auto-anchored with `^`/`$` for full-string matching
- **CIDR host filtering** — host patterns support CIDR notation (e.g., `10.0.0.0/8`) alongside regex; auto-detected
- **Filename validation** — `ValidateFilename()` rejects names >255 chars, control characters, path separators
- **Sudo disabled by default** — requires `--enable-sudo`
- **File permissions preserved** — rwx bits are read from source and applied to destination
- **Remote path expansion** — `~` and relative paths expanded via `sftp.RealPath()` server-side
- **Text output** — handlers return human-readable text via `textResult()` instead of JSON for better UX
- **Efficient directory traversal** — uses `sftp.Walk()` for optimal performance
- **Remote OS detection** — auto-detects OS, architecture, and shell on connect via POSIX probe with Windows fallback; best-effort with 5s timeout; results stored on `Connection` and exposed in `ssh_connect`/`ssh_list_sessions` output

### Package Structure

- `internal/config` — CLI flag/env parsing via `go-arg`, config structs, validation
- `internal/connection` — SSH auth discovery, connection pool with auto-reconnect, remote OS/shell detection
- `internal/security` — host/command filter (regex + CIDR, auto-anchored), rate limiter (token bucket, with cleanup), path traversal check, filename validation, local path validation
- `internal/sshclient` — SFTP operations wrapper (upload/download/list/stat/rename/walk)
- `internal/tools` — input/output types and handlers for all 12 MCP tools
- `internal/server` — MCP server setup, tool registration with annotations, transports

### MCP SDK Usage

Uses `github.com/modelcontextprotocol/go-sdk/mcp` for MCP server implementation. Key patterns:

```go
// Handler signature
func(ctx context.Context, _ *mcp.CallToolRequest, input tools.Input) (*mcp.CallToolResult, any, error)

// Tool registration
mcp.AddTool(s.mcpServer, &mcp.Tool{
    Name:        "tool_name",
    Description: "Tool description",
    Annotations: &mcp.ToolAnnotations{
        Title:           "Display Name",
        ReadOnlyHint:    true,  // bool
        DestructiveHint: boolPtr(false),  // *bool
        IdempotentHint:  true,  // bool
        OpenWorldHint:   boolPtr(true),  // *bool
    },
}, handlerFunc)

// Return text-only result (preferred for Claude Code)
func textResult(text string) *mcp.CallToolResult {
    return &mcp.CallToolResult{
        Content: []mcp.Content{&mcp.TextContent{Text: text}},
    }
}
// Return nil for structured output to force text display
return textResult(out.Text()), nil, nil
```

**Important notes:**
- `jsonschema` struct tags use plain descriptions (not `description=...`)
- Return `nil` as second value to skip structured output (forces text-only in Claude Code)
- `ReadOnlyHint` and `IdempotentHint` are `bool`
- `DestructiveHint` and `OpenWorldHint` are `*bool` (use helper `boolPtr()`)

### CLI Configuration (go-arg)

```go
type Args struct {
    EnableHTTP bool `arg:"--enable-http,env:MCP_SSH_ENABLE_HTTP" help:"enable HTTP transport"`
    HTTPPort   int  `arg:"--http-port,env:MCP_SSH_HTTP_PORT" default:"8081" placeholder:"PORT" help:"HTTP transport port"`
}

// Description shown in --help header
func (Args) Description() string {
    return "SSH MCP Server - provides AI agents with secure SSH access to remote hosts"
}

// Version shown with --version
func (Args) Version() string {
    return "ssh-mcp " + Version
}

// Help output
p.WriteHelp(os.Stdout)  // Full help with descriptions
p.WriteUsage(os.Stdout) // Short usage line only
```

**Placeholder tags** make help clearer: `placeholder:"PORT"` → `--http-port PORT` (not `--http-port HTTP-PORT`)

### SFTP Operations

```go
// Path expansion (server-side, handles ~, .., relative paths)
realPath := sshclient.ExpandRemotePath(sftpClient, "~/config.yaml")

// File operations
sshclient.UploadFile(sftp, local, remote, perms)   // Preserves permissions
sshclient.DownloadFile(sftp, remote, local)        // Preserves permissions
sshclient.ReadFile(sftp, remote)                   // Read content (optional maxSize variadic)
sshclient.ReadFile(sftp, remote, maxSize)          // Read with size limit
sshclient.WriteFile(sftp, remote, data, perms)     // Write with permissions

// File info and rename
sftpClient.Stat(path)      // Follow symlinks
sftpClient.Lstat(path)     // Don't follow symlinks
sftpClient.Rename(old, new)

// Directory operations
sshclient.ListDir(sftp, path)                      // List entries
sshclient.UploadDir(sftp, localDir, remoteDir)     // Recursive upload
sshclient.DownloadDir(sftp, remoteDir, localDir)   // Recursive download

// Efficient directory traversal
walker := sftpClient.Walk(dirPath)
for walker.Step() {
    if err := walker.Err(); err != nil {
        return err
    }
    path := walker.Path()
    stat := walker.Stat()
    // Process entry...
}
```

## Testing

Unit tests are in `*_test.go` files alongside source:
- `config_test.go` — config building, validation, defaults, CLI parsing, new security flags
- `auth_test.go` — host parsing, auth method discovery, missing known_hosts error
- `pool_test.go` — pool operations, session management
- `detect_test.go` — remote OS/shell detection parsing (POSIX and Windows), concurrency safety
- `filter_test.go` — host/command allow/deny with regex, CIDR matching, auto-anchoring, partial match prevention
- `ratelimit_test.go` — per-host rate limiting, burst, cleanup
- `pathcheck_test.go` — path traversal detection, filename validation (length, control chars), local path validation, null bytes, base dir containment
- `server_test.go` — server creation, tool registration, HTTP auth middleware

E2E tests in `tests/e2e/` use testcontainers-go with a Docker SSH server:
- `tests/e2e/e2e_test.go` — all E2E test scenarios (connect, execute, file/dir ops, edit, stat, rename, sessions)
- `tests/e2e/setup_test.go` — Docker container + MCP server setup helpers
- `tests/e2e/Dockerfile` — Ubuntu SSH server image for testing

Run tests:
```bash
go test ./internal/...                   # Unit tests only
go test -v -timeout 120s ./tests/e2e/... # E2E tests (requires Docker)
go test ./...                            # All tests
go test -v ./internal/...                # Verbose unit tests
go test -race ./internal/...             # Race detector
```

## Dependencies

- `github.com/modelcontextprotocol/go-sdk` v1.2.0 — MCP protocol
- `golang.org/x/crypto/ssh` — SSH client
- `github.com/pkg/sftp` v1.13.10 — SFTP client
- `github.com/kevinburke/ssh_config` — SSH config parsing
- `github.com/acarl005/stripansi` — ANSI escape code stripping
- `golang.org/x/time/rate` — rate limiting
- `github.com/alexflint/go-arg` v1.6.1 — CLI argument parsing
- `github.com/testcontainers/testcontainers-go` v0.40.0 — E2E test infrastructure (test only)

## Common Tasks

### Adding a New Tool

1. **Define types** in `internal/tools/types.go`:
   ```go
   type SSHNewToolInput struct {
       SessionID string `json:"session_id" jsonschema:"Session ID from ssh_connect"`
       Param     string `json:"param" jsonschema:"Parameter description"`
   }

   type SSHNewToolOutput struct {
       Result string `json:"result"`
   }

   func (o SSHNewToolOutput) Text() string {
       return o.Result
   }
   ```

2. **Create handler** in `internal/tools/new_tool.go`:
   ```go
   type NewToolDeps struct {
       Pool *connection.Pool
   }

   func HandleNewTool(ctx context.Context, deps *NewToolDeps, input SSHNewToolInput) (*SSHNewToolOutput, error) {
       // Validate, get connection, execute...
       return &SSHNewToolOutput{Result: "success"}, nil
   }
   ```

3. **Register tool** in `internal/server/server.go`:
   ```go
   newToolDeps := &tools.NewToolDeps{Pool: s.pool}

   mcp.AddTool(s.mcpServer, &mcp.Tool{
       Name:        "ssh_new_tool",
       Description: "Tool description",
       Annotations: &mcp.ToolAnnotations{
           Title:           "Display Name",
           ReadOnlyHint:    true,
           DestructiveHint: boolPtr(false),
           IdempotentHint:  true,
           OpenWorldHint:   boolPtr(false),
       },
   }, func(ctx context.Context, _ *mcp.CallToolRequest, input tools.SSHNewToolInput) (*mcp.CallToolResult, any, error) {
       out, err := tools.HandleNewTool(ctx, newToolDeps, input)
       if err != nil {
           return nil, nil, err
       }
       return textResult(out.Text()), nil, nil
   })
   ```

4. **Update README.md** with tool documentation

### Debugging

```bash
# Enable verbose logging (add to main.go or use env var)
export DEBUG=1

# Test with MCP Inspector (if available)
# Or use stdio directly:
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | ./ssh-mcp

# Check connection
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"ssh_connect","arguments":{"host":"example.com"}}}' | ./ssh-mcp
```

### Security Considerations

- Path traversal protection checks for `..` and null bytes in raw paths **before** cleaning
- Local path validation via `ValidateLocalPath()` enforces `--local-base-dir` containment
- Host/command filters use denylist-first priority with auto-anchored regex patterns (`^`/`$`) and optional CIDR matching
- `ValidateFilename()` rejects filenames >255 chars, control characters (0x00-0x1F), path separators, and `..`
- `ValidatePath()` calls `ValidateFilename()` on the base name, so all callers get filename validation automatically
- Command filter runs on the **final** command (after cd/sudo prepend), not just the raw input
- Rate limiter uses per-host token buckets with periodic cleanup of stale entries
- File ops rate limiting is opt-in via `--rate-limit-file-ops`
- Sudo disabled by default, requires explicit flag
- HTTP transport binds to localhost only (hardcoded)
- HTTP transport supports optional bearer token auth via `--http-token`
- Host key verification enabled by default; fails with clear error if `known_hosts` is missing (no silent downgrade)
- Passwords are not stored in the connection pool; only `ssh.ClientConfig` is retained for auto-reconnect
- Connection pool enforces `--max-connections` limit
- `ReadFile` supports optional `maxSize` parameter to prevent memory exhaustion
- `FollowSymlinks` input uses `*bool` to correctly distinguish between "not set" (default true) and "set to false"
- DRY helper `getConnectionWithRateLimit()` used by all file/dir handlers
