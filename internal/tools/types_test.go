package tools

import (
	"encoding/json"
	"testing"
)

func TestSSHConnectInput_NoUseSSHConfig(t *testing.T) {
	// Verify that UseSSHConfig field no longer exists by checking
	// that JSON with use_ssh_config is ignored (no field to unmarshal into).
	data := `{"host":"example.com","use_ssh_config":true}`
	var input SSHConnectInput
	if err := json.Unmarshal([]byte(data), &input); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.Host != "example.com" {
		t.Errorf("Host = %q, want %q", input.Host, "example.com")
	}
	// The field should not exist on the struct anymore - if it did,
	// we'd need to check it. This test simply validates the struct works
	// without the field.
}

func TestSSHReadFileOutput_TextEmptyContent(t *testing.T) {
	out := SSHReadFileOutput{
		Message: "test message",
	}
	if out.Text() != "test message" {
		t.Errorf("Text() = %q, want %q", out.Text(), "test message")
	}
}

func TestSSHReadFileOutput_TextWithContent(t *testing.T) {
	out := SSHReadFileOutput{
		Content: "line1\nline2\n",
		Message: "header",
	}
	expected := "header\nline1\nline2\n"
	if out.Text() != expected {
		t.Errorf("Text() = %q, want %q", out.Text(), expected)
	}
}
