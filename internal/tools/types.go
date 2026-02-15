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
	SessionID    string `json:"session_id"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	User         string `json:"user"`
	ConnectedAt  string `json:"connected_at"`
	LastUsed     string `json:"last_used"`
	CommandCount int    `json:"command_count"`
	Connected    bool   `json:"connected"`
	OS           string `json:"os,omitempty"`
	Arch         string `json:"arch,omitempty"`
	Shell        string `json:"shell,omitempty"`
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
	}
	return strings.TrimRight(b.String(), "\n")
}

// SSHUploadFileInput is the input for the ssh_upload_file tool.
type SSHUploadFileInput struct {
	SessionID  string `json:"session_id" jsonschema:"Session ID from ssh_connect"`
	LocalPath  string `json:"local_path" jsonschema:"Local file path to upload"`
	RemotePath string `json:"remote_path" jsonschema:"Remote destination path"`
}

// SSHUploadFileOutput is the output for the ssh_upload_file tool.
type SSHUploadFileOutput struct {
	BytesWritten int64  `json:"bytes_written"`
	Message      string `json:"message"`
}

// Text returns a human-readable representation of the upload result.
func (o SSHUploadFileOutput) Text() string {
	return o.Message
}

// SSHDownloadFileInput is the input for the ssh_download_file tool.
type SSHDownloadFileInput struct {
	SessionID  string `json:"session_id" jsonschema:"Session ID from ssh_connect"`
	RemotePath string `json:"remote_path" jsonschema:"Remote file path to download"`
	LocalPath  string `json:"local_path" jsonschema:"Local destination path"`
}

// SSHDownloadFileOutput is the output for the ssh_download_file tool.
type SSHDownloadFileOutput struct {
	BytesRead int64  `json:"bytes_read"`
	Message   string `json:"message"`
}

// Text returns a human-readable representation of the download result.
func (o SSHDownloadFileOutput) Text() string {
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

// SSHListDirectoryInput is the input for the ssh_list_directory tool.
type SSHListDirectoryInput struct {
	SessionID string `json:"session_id" jsonschema:"Session ID from ssh_connect"`
	Path      string `json:"path" jsonschema:"Remote directory path to list"`
}

// SSHListDirectoryOutput is the output for the ssh_list_directory tool.
type SSHListDirectoryOutput struct {
	Entries []sshclient.FileEntry `json:"entries"`
	Count   int                   `json:"count"`
}

// Text returns a human-readable representation of the directory listing.
func (o SSHListDirectoryOutput) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d entries:\n", o.Count)
	for _, e := range o.Entries {
		if e.IsDir {
			fmt.Fprintf(&b, "  %s  %s/\n", e.Mode, e.Name)
		} else {
			fmt.Fprintf(&b, "  %s  %8d  %s\n", e.Mode, e.Size, e.Name)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// SSHUploadDirectoryInput is the input for the ssh_upload_directory tool.
type SSHUploadDirectoryInput struct {
	SessionID  string `json:"session_id" jsonschema:"Session ID from ssh_connect"`
	LocalPath  string `json:"local_path" jsonschema:"Local directory path to upload"`
	RemotePath string `json:"remote_path" jsonschema:"Remote destination directory path"`
}

// SSHUploadDirectoryOutput is the output for the ssh_upload_directory tool.
type SSHUploadDirectoryOutput struct {
	FilesUploaded int    `json:"files_uploaded"`
	BytesWritten  int64  `json:"bytes_written"`
	Message       string `json:"message"`
}

// Text returns a human-readable representation of the upload directory result.
func (o SSHUploadDirectoryOutput) Text() string {
	return o.Message
}

// SSHDownloadDirectoryInput is the input for the ssh_download_directory tool.
type SSHDownloadDirectoryInput struct {
	SessionID  string `json:"session_id" jsonschema:"Session ID from ssh_connect"`
	RemotePath string `json:"remote_path" jsonschema:"Remote directory path to download"`
	LocalPath  string `json:"local_path" jsonschema:"Local destination directory path"`
}

// SSHDownloadDirectoryOutput is the output for the ssh_download_directory tool.
type SSHDownloadDirectoryOutput struct {
	FilesDownloaded int    `json:"files_downloaded"`
	BytesRead       int64  `json:"bytes_read"`
	Message         string `json:"message"`
}

// Text returns a human-readable representation of the download directory result.
func (o SSHDownloadDirectoryOutput) Text() string {
	return o.Message
}

// SSHFileStatInput is the input for the ssh_file_stat tool.
type SSHFileStatInput struct {
	SessionID      string `json:"session_id" jsonschema:"Session ID from ssh_connect"`
	RemotePath     string `json:"remote_path" jsonschema:"Remote file or directory path"`
	FollowSymlinks *bool  `json:"follow_symlinks,omitempty" jsonschema:"Optional. Follow symbolic links (default true)"`
}

// SSHFileStatOutput is the output for the ssh_file_stat tool.
type SSHFileStatOutput struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Mode      string `json:"mode"`
	IsDir     bool   `json:"is_dir"`
	IsSymlink bool   `json:"is_symlink"`
	ModTime   string `json:"mod_time"`
}

// Text returns a human-readable representation of the file stat result.
func (o SSHFileStatOutput) Text() string {
	typeStr := "file"
	if o.IsDir {
		typeStr = "directory"
	} else if o.IsSymlink {
		typeStr = "symlink"
	}
	return fmt.Sprintf("%s: %s, size: %d, mode: %s, modified: %s", typeStr, o.Path, o.Size, o.Mode, o.ModTime)
}

// SSHRenameInput is the input for the ssh_rename tool.
type SSHRenameInput struct {
	SessionID string `json:"session_id" jsonschema:"Session ID from ssh_connect"`
	OldPath   string `json:"old_path" jsonschema:"Current path (source)"`
	NewPath   string `json:"new_path" jsonschema:"New path (destination)"`
}

// SSHRenameOutput is the output for the ssh_rename tool.
type SSHRenameOutput struct {
	Message string `json:"message"`
}

// Text returns a human-readable representation of the rename result.
func (o SSHRenameOutput) Text() string {
	return o.Message
}
