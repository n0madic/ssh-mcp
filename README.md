# SSH MCP Server

A Model Context Protocol (MCP) server that provides AI agents with secure SSH access to remote hosts. Supports connection pooling, key-based and password authentication, sudo, file operations via SFTP, command filtering, rate limiting, and graceful shutdown.

## Features

- **SSH Connection Pool** — reuses connections, auto-reconnect on failure, idle cleanup, auto-detection of remote OS and shell
- **Authentication** — SSH keys (auto-discovery), password, `~/.ssh/config` resolution
- **Command Execution** — with sudo support, working directory, timeout, ANSI stripping
- **SFTP File Operations** — upload/download files and directories, edit files (replace/patch), list directories, file stat/rename, `~` path expansion
- **Security** — host/command allowlist/denylist (regex + CIDR), per-host rate limiting, path traversal protection, filename length validation
- **Transports** — stdio (default) and Streamable HTTP (`localhost` only)
- **Graceful Shutdown** — closes all SSH connections on SIGINT/SIGTERM

## Installation

```bash
go install github.com/n0madic/ssh-mcp@latest
```

Or build from source:

```bash
git clone https://github.com/n0madic/ssh-mcp.git
cd ssh-mcp
go build -o ssh-mcp .
```

## Usage

### Stdio transport (default)

```bash
./ssh-mcp
```

### HTTP transport

```bash
./ssh-mcp --enable-http
# Listens on localhost:8081/mcp
```

### Both transports

```bash
./ssh-mcp --enable-http
# Stdio + HTTP on localhost:8081/mcp
```

### HTTP only (no stdio)

```bash
./ssh-mcp --enable-http --disable-stdio
```

## CLI Flags

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--enable-http` | `MCP_SSH_ENABLE_HTTP` | `false` | Enable HTTP transport |
| `--http-port` | `MCP_SSH_HTTP_PORT` | `8081` | HTTP transport port |
| `--disable-stdio` | `MCP_SSH_DISABLE_STDIO` | `false` | Disable stdio transport |
| `--no-verify-host-key` | `MCP_SSH_NO_VERIFY_HOST_KEY` | `false` | Disable host key verification |
| `--known-hosts` | `MCP_SSH_KNOWN_HOSTS` | `~/.ssh/known_hosts` | Path to known_hosts file |
| `--ssh-config` | `MCP_SSH_CONFIG` | `~/.ssh/config` | Path to SSH config file |
| `--enable-sudo` | `MCP_SSH_ENABLE_SUDO` | `false` | Allow sudo execution |
| `--command-timeout` | `MCP_SSH_COMMAND_TIMEOUT` | `60s` | Command execution timeout |
| `--host-allowlist` | `MCP_SSH_HOST_ALLOWLIST` | _(empty)_ | Host allowlist (can be specified multiple times) |
| `--host-denylist` | `MCP_SSH_HOST_DENYLIST` | _(empty)_ | Host denylist (can be specified multiple times) |
| `--command-allowlist` | `MCP_SSH_COMMAND_ALLOWLIST` | _(empty)_ | Command allowlist regex (can be specified multiple times) |
| `--command-denylist` | `MCP_SSH_COMMAND_DENYLIST` | _(empty)_ | Command denylist regex (can be specified multiple times) |
| `--rate-limit` | `MCP_SSH_RATE_LIMIT` | `60` | Rate limit (requests per minute per host) |
| `--rate-limit-file-ops` | `MCP_SSH_RATE_LIMIT_FILE_OPS` | `false` | Apply rate limiting to SFTP file operations |
| `--local-base-dir` | `MCP_SSH_LOCAL_BASE_DIR` | _(empty)_ | Restrict local file operations to this directory |
| `--max-file-size` | `MCP_SSH_MAX_FILE_SIZE` | `0` | Maximum file size for read operations (0=unlimited) |
| `--max-connections` | `MCP_SSH_MAX_CONNECTIONS` | `0` | Maximum concurrent SSH connections (0=unlimited) |
| `--http-token` | `MCP_SSH_HTTP_TOKEN` | _(empty)_ | Bearer token for HTTP transport authentication |
| `--disable-tools` | `MCP_SSH_DISABLE_TOOLS` | _(empty)_ | Disable specific tools (can be specified multiple times) |
| `--version` | — | — | Show version and exit |

**Priority:** CLI flags > environment variables > defaults.

### Examples

**Allow only specific hosts (regex patterns):**
```bash
./ssh-mcp --host-allowlist "192.168.1.*" --host-allowlist "10.0.0.*"
```

**Allow hosts by CIDR range:**
```bash
./ssh-mcp --host-allowlist "10.0.0.0/8" --host-allowlist "192.168.0.0/16"
```

**Block dangerous commands (multiple flags):**
```bash
./ssh-mcp --command-denylist "rm\s+-rf.*" --command-denylist "shutdown.*" --command-denylist "reboot.*"
```

> **Note:** Command/host filter patterns are auto-anchored with `^` and `$` for full-string matching. Use `.*` for substring matching (e.g., `rm\s+-rf.*` matches `rm -rf /` but `rm` alone won't match `format`). Host patterns also support CIDR notation (e.g., `10.0.0.0/8`) — CIDR patterns are detected automatically and match by IP range instead of regex.

**Using environment variables (comma-separated):**
```bash
export MCP_SSH_HOST_ALLOWLIST="host1.example.com,host2.example.com,host3.example.com"
export MCP_SSH_COMMAND_DENYLIST="rm\s+-rf.*,shutdown.*,reboot.*"
export MCP_SSH_ENABLE_SUDO=true
./ssh-mcp
```

**Restrict local file operations to a directory:**
```bash
./ssh-mcp --local-base-dir /tmp/ssh-workspace
```

**Enable HTTP transport with bearer token authentication:**
```bash
./ssh-mcp --enable-http --http-token "my-secret-token"
```

**Limit concurrent connections and file size:**
```bash
./ssh-mcp --max-connections 5 --max-file-size 10485760
```

**Disable specific tools (multiple flags):**
```bash
./ssh-mcp --disable-tools ssh_execute --disable-tools ssh_edit_file
```

**Disable tools via environment variable:**
```bash
export MCP_SSH_DISABLE_TOOLS="ssh_execute,ssh_edit_file,ssh_rename"
./ssh-mcp
```

## MCP Tools

### ssh_connect

Connect to a remote host via SSH.

**Minimal (key auth from ssh-agent or `~/.ssh/id_*`):**
```json
{
  "host": "example.com"
}
```

**Full format — user, password, port inline:**
```json
{
  "host": "admin:secret@example.com:2222"
}
```

**Explicit parameters (override inline values):**
```json
{
  "host": "example.com",
  "port": 2222,
  "user": "admin",
  "key_path": "~/.ssh/id_ed25519"
}
```

**Resolve from `~/.ssh/config`:**
```json
{
  "host": "my-server",
  "use_ssh_config": true
}
```

Returns `session_id` for use with other tools. Also auto-detects remote OS, architecture, and shell.

### ssh_execute

Execute a command on a remote host.

```json
{
  "session_id": "admin@example.com:22",
  "command": "ls -la /var/log",
  "timeout": 30,
  "sudo": true,
  "sudo_password": "secret",
  "working_dir": "/home/admin"
}
```

### ssh_disconnect

Disconnect an SSH session.

```json
{
  "session_id": "admin@example.com:22"
}
```

### ssh_list_sessions

List all active SSH sessions (no parameters required).

### ssh_upload_file

Upload a local file to a remote host via SFTP. Preserves file permissions. Supports `~` for remote home directory.

```json
{
  "session_id": "admin@example.com:22",
  "local_path": "/tmp/config.yaml",
  "remote_path": "~/config.yaml"
}
```

### ssh_download_file

Download a file from a remote host via SFTP. Preserves file permissions. Supports `~` for remote home directory.

```json
{
  "session_id": "admin@example.com:22",
  "remote_path": "~/app.log",
  "local_path": "/tmp/app.log"
}
```

### ssh_edit_file

Edit a file on a remote host. Two modes:

**Replace mode** (default) — full content replacement:
```json
{
  "session_id": "admin@example.com:22",
  "remote_path": "/etc/myapp/config.yaml",
  "mode": "replace",
  "content": "new file content here",
  "backup": true
}
```

**Patch mode** — find and replace:
```json
{
  "session_id": "admin@example.com:22",
  "remote_path": "/etc/myapp/config.yaml",
  "mode": "patch",
  "old_string": "old_value",
  "new_string": "new_value",
  "backup": true
}
```

### ssh_list_directory

List contents of a remote directory. Supports `~` for remote home directory.

```json
{
  "session_id": "admin@example.com:22",
  "path": "~/"
}
```

### ssh_upload_directory

Recursively upload a local directory. Preserves directory structure and permissions.

```json
{
  "session_id": "admin@example.com:22",
  "local_path": "/tmp/myapp",
  "remote_path": "/opt/myapp"
}
```

### ssh_download_directory

Recursively download a remote directory. Preserves directory structure and permissions.

```json
{
  "session_id": "admin@example.com:22",
  "remote_path": "/opt/myapp",
  "local_path": "/tmp/myapp-backup"
}
```

### ssh_file_stat

Get file or directory information (size, permissions, modification time). Supports `~` for remote home directory.

```json
{
  "session_id": "admin@example.com:22",
  "remote_path": "~/config.yaml",
  "follow_symlinks": true
}
```

Returns file metadata including size, permissions (mode), modification time, and whether it's a directory or symlink.

### ssh_rename

Rename or move a file/directory on the remote host. Supports `~` for paths.

```json
{
  "session_id": "admin@example.com:22",
  "old_path": "~/old-name.txt",
  "new_path": "~/new-name.txt"
}
```

Can be used for both renaming files in place and moving files to different directories.

## Claude Code Configuration

Add to Claude Code using the CLI:

```bash
claude mcp add --transport stdio --scope user ssh -- /path/to/ssh-mcp --no-verify-host-key
```

Or manually edit `~/.claude.json`:

```json
{
  "mcpServers": {
    "ssh": {
      "type": "stdio",
      "command": "/path/to/ssh-mcp",
      "args": ["--no-verify-host-key"]
    }
  }
}
```

## Claude Desktop Configuration

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

### Stdio transport

```json
{
  "mcpServers": {
    "ssh": {
      "command": "/path/to/ssh-mcp",
      "args": ["--enable-sudo", "--no-verify-host-key"]
    }
  }
}
```

### HTTP transport

Start the server manually:
```bash
./ssh-mcp --enable-http --disable-stdio
```

Then configure Claude Desktop to use the HTTP endpoint at `http://localhost:8081/mcp`.

## Security

- **HTTP transport is localhost-only** — the HTTP server binds to `localhost` (hardcoded, not configurable)
- **HTTP authentication** — optional bearer token authentication for HTTP transport (`--http-token`)
- **Host key verification** — enabled by default using `~/.ssh/known_hosts`; fails with a clear error if the file is missing (no silent downgrade to insecure mode)
- **Sudo disabled by default** — must be explicitly enabled with `--enable-sudo`
- **Host filtering** — allowlist/denylist with regex and CIDR support; denylist takes priority; regex patterns are auto-anchored for full-string matching; CIDR patterns (e.g., `10.0.0.0/8`) match by IP range
- **Command filtering** — allowlist/denylist with regex support; denylist takes priority; patterns are auto-anchored; filter runs on the final command (after cd/sudo prepend)
- **Local path restriction** — `--local-base-dir` restricts all local file operations (upload/download) to a specific directory
- **Path traversal protection** — rejects paths containing `..` or null bytes (both local and remote)
- **Filename validation** — rejects filenames longer than 255 characters, containing control characters, or path separators
- **Rate limiting** — per-host token bucket rate limiter with automatic stale entry cleanup; optionally applies to SFTP file operations (`--rate-limit-file-ops`)
- **Connection pool limits** — `--max-connections` caps the number of concurrent SSH connections
- **File size limits** — `--max-file-size` caps remote file read operations to prevent memory exhaustion
- **No credential persistence** — passwords are not stored in the connection pool; only the SSH client config (with key-based auth methods) is retained for auto-reconnect
- **Remote path expansion** — `~` expands to user's home directory on remote server

## Development

```bash
# Build
go build ./...

# Run unit tests
go test ./internal/...

# Run E2E tests (requires Docker)
go test -v -timeout 120s ./e2e/...

# Run all tests
go test ./...

# Vet
go vet ./...
```

## License
MIT License. See [LICENSE](LICENSE) for details.
