package tools

import "testing"

func TestSSHReadFileOutputText_WithContent(t *testing.T) {
	out := SSHReadFileOutput{
		Content:    "     1\thello\n     2\tworld\n",
		TotalLines: 2,
		FileSize:   12,
		FromLine:   1,
		ToLine:     2,
		Message:    "/tmp/test.txt: showing lines 1-2 of 2 (12 bytes)",
	}

	result := out.Text()
	expected := "/tmp/test.txt: showing lines 1-2 of 2 (12 bytes)\n     1\thello\n     2\tworld\n"
	if result != expected {
		t.Errorf("Text() = %q, want %q", result, expected)
	}
}

func TestSSHReadFileOutputText_EmptyFile(t *testing.T) {
	out := SSHReadFileOutput{
		TotalLines: 0,
		FileSize:   0,
		Message:    "/tmp/empty.txt: 0 lines, 0 bytes",
	}

	result := out.Text()
	expected := "/tmp/empty.txt: 0 lines, 0 bytes"
	if result != expected {
		t.Errorf("Text() = %q, want %q", result, expected)
	}
}

func TestSSHReadFileOutputText_OffsetBeyondEOF(t *testing.T) {
	out := SSHReadFileOutput{
		TotalLines: 5,
		FileSize:   100,
		FromLine:   10,
		ToLine:     9,
		Message:    "/tmp/test.txt: offset 10 is beyond end of file (5 lines, 100 bytes)",
	}

	result := out.Text()
	// When Content is empty, Text() should return just the message.
	if result != out.Message {
		t.Errorf("Text() = %q, want %q", result, out.Message)
	}
}
