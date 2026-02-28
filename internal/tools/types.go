package tools

import (
	"fmt"
	"strings"

	"github.com/n0madic/ssh-mcp/internal/sshclient"
)

// SSHConnectInput is the input for the ssh_connect tool.
type SSHConnectInput struct {
	Host         string `json:"host" jsonschema:"Required. SSH host — hostname, host:port, user@host, or user:password@host:port. This is the only required field, all others are optional and auto-discovered."`
	Port         int    `json:"port,omitempty" jsonschema:"Optional. SSH port override (default 22)"`
	User         string `json:"user,omitempty" jsonschema:"Optional. SSH username override (default: current OS user)"`
	Password     string `json:"password,omitempty" jsonschema:"Optional. SSH password override"`
	KeyPath      string `json:"key_path,omitempty" jsonschema:"Optional. Path to SSH private key (default: auto-discovered from ~/.ssh/)"`
	UseSSHConfig bool   `json:"use_ssh_config,omitempty" jsonschema:"Optional. Resolve host alias from ~/.ssh/config"`
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

// SSHFileInfoInput is the input for the ssh_file_info tool.
type SSHFileInfoInput struct {
	SessionID      string `json:"session_id" jsonschema:"Session ID from ssh_connect"`
	RemotePath     string `json:"remote_path" jsonschema:"Remote file or directory path"`
	FollowSymlinks *bool  `json:"follow_symlinks,omitempty" jsonschema:"Optional. Follow symbolic links (default true)"`
	StatOnly       *bool  `json:"stat_only,omitempty" jsonschema:"Optional. For directories: return only stat info without listing contents (default false)"`
}

// SSHFileInfoOutput is the output for the ssh_file_info tool.
type SSHFileInfoOutput struct {
	Name      string                `json:"name"`
	Path      string                `json:"path"`
	Size      int64                 `json:"size"`
	Mode      string                `json:"mode"`
	IsDir     bool                  `json:"is_dir"`
	IsSymlink bool                  `json:"is_symlink"`
	ModTime   string                `json:"mod_time"`
	Entries   []sshclient.FileEntry `json:"entries,omitempty"`
}

// Text returns a human-readable representation of the file info result.
func (o SSHFileInfoOutput) Text() string {
	typeStr := "file"
	if o.IsDir {
		typeStr = "directory"
	} else if o.IsSymlink {
		typeStr = "symlink"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s, size: %d, mode: %s, modified: %s", typeStr, o.Path, o.Size, o.Mode, o.ModTime)
	if len(o.Entries) > 0 {
		fmt.Fprintf(&b, "\n%d entries:\n", len(o.Entries))
		for _, e := range o.Entries {
			if e.IsDir {
				fmt.Fprintf(&b, "  %s  %s/\n", e.Mode, e.Name)
			} else {
				fmt.Fprintf(&b, "  %s  %8d  %s\n", e.Mode, e.Size, e.Name)
			}
		}
	}
	return strings.TrimRight(b.String(), "\n")
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
