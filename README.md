# SSH MCP Server

A Model Context Protocol (MCP) server that provides AI agents with secure SSH access to remote hosts. Supports connection pooling, key-based and password authentication, sudo, file operations via SFTP, command filtering, rate limiting, and graceful shutdown.

## Features

- **SSH Connection Pool** — reuses connections, auto-reconnect on failure, idle cleanup, auto-detection of remote OS and shell
- **Authentication** — SSH keys (auto-discovery), password, automatic `~/.ssh/config` alias resolution
- **Command Execution** — with sudo support, working directory, timeout, graceful kill (SIGTERM → SIGKILL), ANSI stripping
- **SFTP File Operations** — upload/download files and directories, read files with line offset/limit, edit files (replace/patch/create), file info with directory listing, `~` path expansion
- **Interactive PTY Terminals** — buffered PTY sessions for interactive programs (vim, htop, REPL), dialogs, and real-time output (opt-in with `--enable-terminal`)
- **Security** — host/command allowlist/denylist (regex + CIDR), per-host rate limiting, path traversal protection, filename length validation
- **Transports** — stdio (default) and Streamable HTTP (`localhost` only)
- **Graceful Shutdown** — closes all SSH connections and terminal sessions on SIGINT/SIGTERM

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
| `--enable-terminal` | `MCP_SSH_ENABLE_TERMINAL` | `false` | Allow interactive PTY terminal sessions (`ssh_open_terminal`) |
| `--max-terminals` | `MCP_SSH_MAX_TERMINALS` | `0` | Maximum concurrent PTY terminal sessions (0=unlimited) |
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
export MCP_SSH_DISABLE_TOOLS="ssh_execute,ssh_edit_file"
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

**SSH config alias (resolved automatically from `~/.ssh/config`):**
```json
{
  "host": "my-server"
}
```

SSH config aliases are resolved automatically — no extra flags needed. Explicit parameters (port, user, key_path) override values from the config.

Returns `session_id` for use with other tools. Also auto-detects remote OS, architecture, and shell.

### ssh_execute

Execute a command on a remote host. On timeout, sends SIGTERM first (5s grace period) then SIGKILL, and returns partial stdout/stderr with a `[TIMEOUT]` marker in stderr.

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

List all active SSH sessions with their connection details, statistics, and active terminal sessions (no parameters required).

### ssh_upload

Upload a local file or directory to a remote host via SFTP. Automatically detects whether the local path is a file or directory. Preserves file permissions and directory structure. Supports `~` for remote home directory.

**Upload a file:**
```json
{
  "session_id": "admin@example.com:22",
  "local_path": "/tmp/config.yaml",
  "remote_path": "~/config.yaml"
}
```

**Upload a directory (recursive):**
```json
{
  "session_id": "admin@example.com:22",
  "local_path": "/tmp/myapp",
  "remote_path": "/opt/myapp"
}
```

### ssh_download

Download a file or directory from a remote host via SFTP. Automatically detects whether the remote path is a file or directory. Preserves file permissions and directory structure. Supports `~` for remote home directory.

**Download a file:**
```json
{
  "session_id": "admin@example.com:22",
  "remote_path": "~/app.log",
  "local_path": "/tmp/app.log"
}
```

**Download a directory (recursive):**
```json
{
  "session_id": "admin@example.com:22",
  "remote_path": "/opt/myapp",
  "local_path": "/tmp/myapp-backup"
}
```

### ssh_edit_file

Edit a file on a remote host. Two modes:

**Replace mode** (default) — full content replacement or new file creation:
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

### ssh_file_info

Get file or directory information (size, permissions, modification time). For directories, also lists contents unless `stat_only` is set. Supports `~` for remote home directory.

**File info:**
```json
{
  "session_id": "admin@example.com:22",
  "remote_path": "~/config.yaml",
  "follow_symlinks": true
}
```

**Directory listing (default for directories):**
```json
{
  "session_id": "admin@example.com:22",
  "remote_path": "~/"
}
```

**Directory stat only (no listing):**
```json
{
  "session_id": "admin@example.com:22",
  "remote_path": "~/",
  "stat_only": true
}
```

Returns file metadata including size, permissions (mode), modification time, and whether it's a directory or symlink. For directories, also returns the list of entries.

### ssh_read_file

Read a file from a remote host with optional line offset and limit. Returns content with line numbers (like `cat -n`). Supports `~` for home directory.

**Read entire file:**
```json
{
  "session_id": "admin@example.com:22",
  "remote_path": "~/app.log"
}
```

**Read with offset and limit (pagination):**
```json
{
  "session_id": "admin@example.com:22",
  "remote_path": "/var/log/syslog",
  "offset": 100,
  "limit": 50
}
```

**Read with max file size limit:**
```json
{
  "session_id": "admin@example.com:22",
  "remote_path": "~/large-file.csv",
  "max_size": 1048576
}
```

Returns file content with line numbers, total line count, file size, and which lines are shown.

---

## Interactive PTY Terminal Tools

These four tools provide buffered PTY access for interactive programs. Requires `--enable-terminal`.

**Typical workflow:**

```
ssh_open_terminal   →  opens shell, returns terminal_id + initial prompt
ssh_send_input      →  write text or keystrokes, get new output
ssh_read_output     →  poll for new output without writing
ssh_close_terminal  →  close the session
```

Active terminals are also listed in `ssh_list_sessions` output.

### ssh_open_terminal

Open a PTY terminal session. Requires `--enable-terminal` flag on the server.

```json
{
  "session_id": "admin@example.com:22",
  "cols": 120,
  "rows": 50,
  "term_type": "xterm-256color",
  "wait_ms": 500
}
```

Returns `terminal_id` and the initial shell output (prompt). `cols`/`rows`/`term_type`/`wait_ms` are optional.

### ssh_send_input

Send text or a special key to a terminal and read new output.

**Send a command:**
```json
{
  "terminal_id": "term-1",
  "text": "ls -la\n",
  "wait_ms": 300
}
```

**Send a special key:**
```json
{
  "terminal_id": "term-1",
  "special_key": "CTRL_C"
}
```

Supported special keys: `CTRL_C`, `CTRL_D`, `CTRL_Z`, `ESC`, `TAB`, `BACKSPACE`, `ENTER`, `ARROW_UP`, `ARROW_DOWN`, `ARROW_LEFT`, `ARROW_RIGHT`.

Use `\n` for newline and `\r` for carriage return in `text`. Exactly one of `text` or `special_key` must be provided — setting both is an error.

### ssh_read_output

Read buffered output since the last read without writing anything.

```json
{
  "terminal_id": "term-1",
  "wait_ms": 1000
}
```

Set `wait_ms` to wait up to N milliseconds for new data (default 0 = return immediately).

### ssh_close_terminal

Close a PTY terminal session.

```json
{
  "terminal_id": "term-1"
}
```

---

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
- **Interactive terminals disabled by default** — PTY sessions bypass the command filter; must be explicitly enabled with `--enable-terminal`
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
