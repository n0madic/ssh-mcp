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
