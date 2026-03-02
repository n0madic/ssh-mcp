package tools

import (
	"fmt"
	"strings"
)

// SSHConnectInput is the input for the ssh_connect tool.
type SSHConnectInput struct {
	Host     string `json:"host" jsonschema:"Required. SSH host — hostname, host:port, user@host, or user:password@host:port. This is the only required field, all others are optional and auto-discovered."`
	Port     int    `json:"port,omitempty" jsonschema:"Optional. SSH port override (default 22)"`
	User     string `json:"user,omitempty" jsonschema:"Optional. SSH username override (default: current OS user)"`
	Password string `json:"password,omitempty" jsonschema:"Optional. SSH password override"`
	KeyPath  string `json:"key_path,omitempty" jsonschema:"Optional. Path to SSH private key (default: auto-discovered from ~/.ssh/)"`
}

// SSHConnectOutput is the output for the ssh_connect tool.
type SSHConnectOutput struct {
	SessionID string `json:"session_id"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	User      string `json:"user"`
	Message   string `json:"message"`
	OS        string `json:"os,omitempty"`
	Arch      string `json:"arch,omitempty"`
	Shell     string `json:"shell,omitempty"`
}

// Text returns a human-readable representation of the connect result.
func (o SSHConnectOutput) Text() string {
	return o.Message
}

// SSHExecuteInput is the input for the ssh_execute tool.
type SSHExecuteInput struct {
	SessionID    string `json:"session_id" jsonschema:"Session ID from ssh_connect"`
	Command      string `json:"command" jsonschema:"Command to execute"`
	Timeout      int    `json:"timeout,omitempty" jsonschema:"Command timeout in seconds (default from config)"`
	Sudo         bool   `json:"sudo,omitempty" jsonschema:"Execute with sudo"`
	SudoPassword string `json:"sudo_password,omitempty" jsonschema:"Password for sudo (command is executed via 'sudo -S sh -c ...')"`
	WorkingDir   string `json:"working_dir,omitempty" jsonschema:"Working directory for command execution"`
}

// SSHExecuteOutput is the output for the ssh_execute tool.
type SSHExecuteOutput struct {
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	DurationMs int64  `json:"duration_ms"`
}

// Text returns a human-readable representation of the execute result.
func (o SSHExecuteOutput) Text() string {
	var b strings.Builder
	if o.Stdout != "" {
		b.WriteString(o.Stdout)
	}
	if o.Stderr != "" {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("[stderr] ")
		b.WriteString(o.Stderr)
	}
	if o.ExitCode != 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "Exit code: %d", o.ExitCode)
	}
	if b.Len() == 0 {
		fmt.Fprintf(&b, "Completed (exit code %d, %dms)", o.ExitCode, o.DurationMs)
	}
	return b.String()
}

// SSHDisconnectInput is the input for the ssh_disconnect tool.
type SSHDisconnectInput struct {
	SessionID string `json:"session_id" jsonschema:"Session ID to disconnect"`
}

// SSHDisconnectOutput is the output for the ssh_disconnect tool.
type SSHDisconnectOutput struct {
	Message string `json:"message"`
}

// Text returns a human-readable representation of the disconnect result.
func (o SSHDisconnectOutput) Text() string {
	return o.Message
}

// SSHListSessionsOutput is the output for the ssh_list_sessions tool.
type SSHListSessionsOutput struct {
	Sessions []SessionInfo `json:"sessions"`
	Count    int           `json:"count"`
}

// SessionInfo provides information about an active session.
type SessionInfo struct {
	SessionID    string               `json:"session_id"`
	Host         string               `json:"host"`
	Port         int                  `json:"port"`
	User         string               `json:"user"`
	ConnectedAt  string               `json:"connected_at"`
	LastUsed     string               `json:"last_used"`
	CommandCount int                  `json:"command_count"`
	Connected    bool                 `json:"connected"`
	OS           string               `json:"os,omitempty"`
	Arch         string               `json:"arch,omitempty"`
	Shell        string               `json:"shell,omitempty"`
	Terminals    []TerminalInfoOutput `json:"terminals,omitempty"`
	Tunnels      []TunnelInfoOutput   `json:"tunnels,omitempty"`
}

// Text returns a human-readable representation of the sessions list.
func (o SSHListSessionsOutput) Text() string {
	if o.Count == 0 {
		return "No active sessions"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Active sessions (%d):\n", o.Count)
	for _, s := range o.Sessions {
		status := "connected"
		if !s.Connected {
			status = "disconnected"
		}
		line := fmt.Sprintf("  %s — %s, %d commands, last used %s", s.SessionID, status, s.CommandCount, s.LastUsed)
		if s.OS != "" {
			detail := s.OS
			if s.Arch != "" {
				detail += " " + s.Arch
			}
			if s.Shell != "" {
				detail += ", " + s.Shell
			}
			line += fmt.Sprintf(" [%s]", detail)
		}
		b.WriteString(line + "\n")
		for _, t := range s.Terminals {
			fmt.Fprintf(&b, "    terminal %s — created %s, last used %s\n", t.TerminalID, t.CreatedAt, t.LastUsed)
		}
		for _, t := range s.Tunnels {
			fmt.Fprintf(&b, "    tunnel %s — %s → %s (%d connections)\n", t.TunnelID, t.LocalAddr, t.RemoteAddr, t.ConnCount)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// SSHUploadInput is the input for the ssh_upload tool.
type SSHUploadInput struct {
	SessionID  string `json:"session_id" jsonschema:"Session ID from ssh_connect"`
	LocalPath  string `json:"local_path" jsonschema:"Local file or directory path to upload"`
	RemotePath string `json:"remote_path" jsonschema:"Remote destination path"`
}

// SSHUploadOutput is the output for the ssh_upload tool.
type SSHUploadOutput struct {
	FilesUploaded int    `json:"files_uploaded"`
	BytesWritten  int64  `json:"bytes_written"`
	Message       string `json:"message"`
}

// Text returns a human-readable representation of the upload result.
func (o SSHUploadOutput) Text() string {
	return o.Message
}

// SSHDownloadInput is the input for the ssh_download tool.
type SSHDownloadInput struct {
	SessionID  string `json:"session_id" jsonschema:"Session ID from ssh_connect"`
	RemotePath string `json:"remote_path" jsonschema:"Remote file or directory path to download"`
	LocalPath  string `json:"local_path" jsonschema:"Local destination path"`
}

// SSHDownloadOutput is the output for the ssh_download tool.
type SSHDownloadOutput struct {
	FilesDownloaded int    `json:"files_downloaded"`
	BytesRead       int64  `json:"bytes_read"`
	Message         string `json:"message"`
}

// Text returns a human-readable representation of the download result.
func (o SSHDownloadOutput) Text() string {
	return o.Message
}

// SSHEditFileInput is the input for the ssh_edit_file tool.
type SSHEditFileInput struct {
	SessionID  string `json:"session_id" jsonschema:"Session ID from ssh_connect"`
	RemotePath string `json:"remote_path" jsonschema:"Remote file path to edit"`
	Mode       string `json:"mode,omitempty" jsonschema:"Edit mode: replace (full content) or patch (find and replace)"`
	Content    string `json:"content,omitempty" jsonschema:"Full file content (for replace mode)"`
	OldString  string `json:"old_string,omitempty" jsonschema:"String to find (for patch mode)"`
	NewString  string `json:"new_string,omitempty" jsonschema:"String to replace with (for patch mode)"`
	Backup     *bool  `json:"backup,omitempty" jsonschema:"Create .bak backup before editing (default true)"`
}

// SSHEditFileOutput is the output for the ssh_edit_file tool.
type SSHEditFileOutput struct {
	BytesWritten int64  `json:"bytes_written"`
	Message      string `json:"message"`
}

// Text returns a human-readable representation of the edit result.
func (o SSHEditFileOutput) Text() string {
	return o.Message
}

// SSHReadFileInput is the input for the ssh_read_file tool.
type SSHReadFileInput struct {
	SessionID  string `json:"session_id" jsonschema:"Session ID from ssh_connect"`
	RemotePath string `json:"remote_path" jsonschema:"Remote file path to read"`
	Offset     int    `json:"offset,omitempty" jsonschema:"Line offset to start reading from (1-based, default 1)"`
	Limit      int    `json:"limit,omitempty" jsonschema:"Maximum number of lines to return (default 0 = all lines)"`
	MaxSize    int64  `json:"max_size,omitempty" jsonschema:"Maximum file size in bytes (default from server config, 0=unlimited)"`
}

// SSHReadFileOutput is the output for the ssh_read_file tool.
type SSHReadFileOutput struct {
	Content    string `json:"content"`
	TotalLines int    `json:"total_lines"`
	FileSize   int64  `json:"file_size"`
	FromLine   int    `json:"from_line"`
	ToLine     int    `json:"to_line"`
	Message    string `json:"message"`
}

// Text returns a human-readable representation of the read file result.
func (o SSHReadFileOutput) Text() string {
	if o.Content == "" {
		return o.Message
	}
	return o.Message + "\n" + o.Content
}

// SSHOpenTerminalInput is the input for the ssh_open_terminal tool.
type SSHOpenTerminalInput struct {
	SessionID string `json:"session_id" jsonschema:"Session ID from ssh_connect"`
	Cols      int    `json:"cols,omitempty" jsonschema:"Terminal width in columns (default 120)"`
	Rows      int    `json:"rows,omitempty" jsonschema:"Terminal height in rows (default 50)"`
	TermType  string `json:"term_type,omitempty" jsonschema:"Terminal type (default xterm-256color)"`
	WaitMs    int    `json:"wait_ms,omitempty" jsonschema:"Milliseconds to wait for initial output (default 500)"`
}

// SSHOpenTerminalOutput is the output for the ssh_open_terminal tool.
type SSHOpenTerminalOutput struct {
	TerminalID string `json:"terminal_id"`
	Output     string `json:"output"`
	Message    string `json:"message"`
}

// Text returns a human-readable representation of the open terminal result.
func (o SSHOpenTerminalOutput) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Terminal opened: %s\n%s", o.TerminalID, o.Message)
	if o.Output != "" {
		b.WriteString("\n")
		b.WriteString(o.Output)
	}
	return b.String()
}

// SSHSendInputInput is the input for the ssh_send_input tool.
type SSHSendInputInput struct {
	TerminalID string `json:"terminal_id" jsonschema:"Terminal ID from ssh_open_terminal"`
	Text       string `json:"text,omitempty" jsonschema:"Text to send (use \\n for newline, \\r for carriage return). Note: literal backslash-n in the JSON string is expanded to a real newline; there is no way to send a literal backslash-n sequence."`
	SpecialKey string `json:"special_key,omitempty" jsonschema:"Special key: CTRL_C, CTRL_D, CTRL_Z, ESC, TAB, BACKSPACE, ENTER, ARROW_UP, ARROW_DOWN, ARROW_LEFT, ARROW_RIGHT"`
	WaitMs     int    `json:"wait_ms,omitempty" jsonschema:"Milliseconds to wait for output after sending (default 300); increase for slow commands, output is returned directly in the response"`
}

// SSHSendInputOutput is the output for the ssh_send_input tool.
type SSHSendInputOutput struct {
	Output  string `json:"output"`
	Written int    `json:"bytes_written"`
}

// Text returns a human-readable representation of the send input result.
func (o SSHSendInputOutput) Text() string {
	return o.Output
}

// SSHReadOutputInput is the input for the ssh_read_output tool.
type SSHReadOutputInput struct {
	TerminalID string `json:"terminal_id" jsonschema:"Terminal ID from ssh_open_terminal"`
	WaitMs     int    `json:"wait_ms,omitempty" jsonschema:"Max milliseconds to wait for new output (default 0 = return immediately)"`
}

// SSHReadOutputOutput is the output for the ssh_read_output tool.
type SSHReadOutputOutput struct {
	Output string `json:"output"`
	HasNew bool   `json:"has_new"`
}

// Text returns a human-readable representation of the read output result.
func (o SSHReadOutputOutput) Text() string {
	return o.Output
}

// TerminalInfoOutput provides information about an active terminal session.
type TerminalInfoOutput struct {
	TerminalID string `json:"terminal_id"`
	SessionID  string `json:"session_id"`
	CreatedAt  string `json:"created_at"`
	LastUsed   string `json:"last_used"`
}

// SSHCloseTerminalInput is the input for the ssh_close_terminal tool.
type SSHCloseTerminalInput struct {
	TerminalID string `json:"terminal_id" jsonschema:"Terminal ID from ssh_open_terminal"`
}

// SSHCloseTerminalOutput is the output for the ssh_close_terminal tool.
type SSHCloseTerminalOutput struct {
	Message string `json:"message"`
}

// Text returns a human-readable representation of the close terminal result.
func (o SSHCloseTerminalOutput) Text() string {
	return o.Message
}

// SSHTunnelCreateInput is the input for the ssh_tunnel_create tool.
type SSHTunnelCreateInput struct {
	SessionID  string `json:"session_id" jsonschema:"Session ID from ssh_connect"`
	RemoteAddr string `json:"remote_addr" jsonschema:"Remote address to forward to (e.g. localhost:5432, 10.0.0.1:80)"`
	LocalPort  int    `json:"local_port,omitempty" jsonschema:"Local port to listen on (0 = auto-assign a free port)"`
}

// SSHTunnelCreateOutput is the output for the ssh_tunnel_create tool.
type SSHTunnelCreateOutput struct {
	TunnelID   string `json:"tunnel_id"`
	LocalAddr  string `json:"local_addr"`
	LocalPort  int    `json:"local_port"`
	RemoteAddr string `json:"remote_addr"`
	Message    string `json:"message"`
}

// Text returns a human-readable representation of the tunnel create result.
func (o SSHTunnelCreateOutput) Text() string {
	return o.Message
}

// SSHTunnelListInput is the input for the ssh_tunnel_list tool.
type SSHTunnelListInput struct {
	SessionID string `json:"session_id,omitempty" jsonschema:"Optional. Filter tunnels by session ID"`
}

// SSHTunnelListOutput is the output for the ssh_tunnel_list tool.
type SSHTunnelListOutput struct {
	Tunnels []TunnelInfoOutput `json:"tunnels"`
	Count   int                `json:"count"`
}

// Text returns a human-readable representation of the tunnel list result.
func (o SSHTunnelListOutput) Text() string {
	if o.Count == 0 {
		return "No active tunnels"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Active tunnels (%d):\n", o.Count)
	for _, t := range o.Tunnels {
		fmt.Fprintf(&b, "  %s — %s → %s (%d connections, created %s)\n",
			t.TunnelID, t.LocalAddr, t.RemoteAddr, t.ConnCount, t.CreatedAt)
	}
	return strings.TrimRight(b.String(), "\n")
}

// TunnelInfoOutput provides information about an active tunnel.
type TunnelInfoOutput struct {
	TunnelID   string `json:"tunnel_id"`
	SessionID  string `json:"session_id"`
	LocalAddr  string `json:"local_addr"`
	RemoteAddr string `json:"remote_addr"`
	ConnCount  int64  `json:"conn_count"`
	CreatedAt  string `json:"created_at"`
	LastUsed   string `json:"last_used"`
}

// SSHTunnelCloseInput is the input for the ssh_tunnel_close tool.
type SSHTunnelCloseInput struct {
	TunnelID string `json:"tunnel_id" jsonschema:"Tunnel ID from ssh_tunnel_create"`
}

// SSHTunnelCloseOutput is the output for the ssh_tunnel_close tool.
type SSHTunnelCloseOutput struct {
	Message string `json:"message"`
}

// Text returns a human-readable representation of the tunnel close result.
func (o SSHTunnelCloseOutput) Text() string {
	return o.Message
}
