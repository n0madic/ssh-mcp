package tools

import (
	"strings"
	"testing"
)

func TestTruncateOutput_Unlimited(t *testing.T) {
	input := "hello world"
	result := TruncateOutput(input, 0)
	if result != input {
		t.Errorf("expected unchanged output for maxBytes=0, got %q", result)
	}
}

func TestTruncateOutput_Negative(t *testing.T) {
	input := "hello world"
	result := TruncateOutput(input, -1)
	if result != input {
		t.Errorf("expected unchanged output for negative maxBytes, got %q", result)
	}
}

func TestTruncateOutput_ShortString(t *testing.T) {
	input := "short"
	result := TruncateOutput(input, 100)
	if result != input {
		t.Errorf("expected unchanged output for short string, got %q", result)
	}
}

func TestTruncateOutput_ExactLimit(t *testing.T) {
	input := "exact"
	result := TruncateOutput(input, len(input))
	if result != input {
		t.Errorf("expected unchanged output at exact limit, got %q", result)
	}
}

func TestTruncateOutput_OverLimit(t *testing.T) {
	input := "hello world, this is a long string"
	result := TruncateOutput(input, 5)
	if !strings.HasPrefix(result, "hello") {
		t.Errorf("expected truncated output to start with 'hello', got %q", result)
	}
	if !strings.Contains(result, "[OUTPUT TRUNCATED: showing first 5 of 34 bytes]") {
		t.Errorf("expected truncation marker, got %q", result)
	}
}

func TestTruncateOutput_EmptyString(t *testing.T) {
	result := TruncateOutput("", 10)
	if result != "" {
		t.Errorf("expected empty string unchanged, got %q", result)
	}
}

func TestTruncateOutput_UTF8Boundary(t *testing.T) {
	// "Hello, 世界" — 世 is 3 bytes (E4 B8 96), 界 is 3 bytes (E7 95 8C).
	// "Hello, " = 7 bytes, then 世 = 3 bytes, total "Hello, 世" = 10 bytes.
	input := "Hello, 世界"
	// Truncate at 9 bytes — would be in the middle of 世 (E4 B8 96).
	result := TruncateOutput(input, 9)
	// Should back up to byte 7 (after ", ") to avoid splitting UTF-8.
	if strings.Contains(result, "\xE4\xB8") {
		t.Errorf("truncation should not split a UTF-8 character: %q", result)
	}
	if !strings.Contains(result, "[OUTPUT TRUNCATED") {
		t.Errorf("expected truncation marker, got %q", result)
	}
}
