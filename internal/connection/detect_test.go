package connection

import (
	"sync"
	"testing"
)

func TestParseDetectionOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected RemoteInfo
	}{
		{
			name:   "Linux full output",
			output: "Linux\nx86_64\n/bin/bash",
			expected: RemoteInfo{
				OS:    "Linux",
				Arch:  "x86_64",
				Shell: "/bin/bash",
			},
		},
		{
			name:   "Darwin full output",
			output: "Darwin\narm64\n/bin/zsh",
			expected: RemoteInfo{
				OS:    "Darwin",
				Arch:  "arm64",
				Shell: "/bin/zsh",
			},
		},
		{
			name:   "FreeBSD full output",
			output: "FreeBSD\namd64\n/bin/sh",
			expected: RemoteInfo{
				OS:    "FreeBSD",
				Arch:  "amd64",
				Shell: "/bin/sh",
			},
		},
		{
			name:   "Linux aarch64",
			output: "Linux\naarch64\n/bin/bash",
			expected: RemoteInfo{
				OS:    "Linux",
				Arch:  "aarch64",
				Shell: "/bin/bash",
			},
		},
		{
			name:   "partial output - only OS",
			output: "Linux",
			expected: RemoteInfo{
				OS: "Linux",
			},
		},
		{
			name:   "partial output - OS and arch",
			output: "Linux\nx86_64",
			expected: RemoteInfo{
				OS:   "Linux",
				Arch: "x86_64",
			},
		},
		{
			name:     "empty output",
			output:   "",
			expected: RemoteInfo{},
		},
		{
			name:   "extra whitespace",
			output: "  Linux  \n  x86_64  \n  /bin/bash  ",
			expected: RemoteInfo{
				OS:    "Linux",
				Arch:  "x86_64",
				Shell: "/bin/bash",
			},
		},
		{
			name:   "extra lines ignored",
			output: "Linux\nx86_64\n/bin/bash\nextra line\n",
			expected: RemoteInfo{
				OS:    "Linux",
				Arch:  "x86_64",
				Shell: "/bin/bash",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDetectionOutput(tt.output)
			if result != tt.expected {
				t.Errorf("parseDetectionOutput(%q) = %+v, want %+v", tt.output, result, tt.expected)
			}
		})
	}
}

func TestParseWindowsDetectionOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected RemoteInfo
	}{
		{
			name:   "typical Windows output",
			output: "Windows_NT\nAMD64\nC:\\Windows\\system32\\cmd.exe",
			expected: RemoteInfo{
				OS:    "Windows",
				Arch:  "AMD64",
				Shell: "C:\\Windows\\system32\\cmd.exe",
			},
		},
		{
			name:   "Windows x86",
			output: "Windows_NT\nx86\nC:\\Windows\\system32\\cmd.exe",
			expected: RemoteInfo{
				OS:    "Windows",
				Arch:  "x86",
				Shell: "C:\\Windows\\system32\\cmd.exe",
			},
		},
		{
			name:   "Windows ARM64",
			output: "Windows_NT\nARM64\nC:\\Windows\\system32\\cmd.exe",
			expected: RemoteInfo{
				OS:    "Windows",
				Arch:  "ARM64",
				Shell: "C:\\Windows\\system32\\cmd.exe",
			},
		},
		{
			name:   "partial output - only OS",
			output: "Windows_NT",
			expected: RemoteInfo{
				OS: "Windows",
			},
		},
		{
			name:     "empty output",
			output:   "",
			expected: RemoteInfo{},
		},
		{
			name:     "non-Windows output returns empty",
			output:   "Linux\nx86_64\n/bin/bash",
			expected: RemoteInfo{},
		},
		{
			name:   "Windows with extra whitespace",
			output: "  Windows_NT  \n  AMD64  \n  C:\\Windows\\system32\\cmd.exe  ",
			expected: RemoteInfo{
				OS:    "Windows",
				Arch:  "AMD64",
				Shell: "C:\\Windows\\system32\\cmd.exe",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseWindowsDetectionOutput(tt.output)
			if result != tt.expected {
				t.Errorf("parseWindowsDetectionOutput(%q) = %+v, want %+v", tt.output, result, tt.expected)
			}
		})
	}
}

func TestGetRemoteInfoConcurrency(t *testing.T) {
	conn := &Connection{
		RemoteInfo: RemoteInfo{
			OS:    "Linux",
			Arch:  "x86_64",
			Shell: "/bin/bash",
		},
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			info := conn.GetRemoteInfo()
			if info.OS != "Linux" {
				t.Errorf("expected OS=Linux, got %s", info.OS)
			}
			if info.Arch != "x86_64" {
				t.Errorf("expected Arch=x86_64, got %s", info.Arch)
			}
			if info.Shell != "/bin/bash" {
				t.Errorf("expected Shell=/bin/bash, got %s", info.Shell)
			}
		}()
	}
	wg.Wait()
}
