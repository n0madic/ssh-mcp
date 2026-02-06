package security

import (
	"testing"
)

func TestFilter_AllowHost_EmptyLists(t *testing.T) {
	f, err := NewFilter(nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := f.AllowHost("any-host"); err != nil {
		t.Errorf("expected allow all when lists are empty, got: %v", err)
	}
}

func TestFilter_AllowHost_Allowlist(t *testing.T) {
	f, err := NewFilter([]string{"host1", "host2"}, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := f.AllowHost("host1"); err != nil {
		t.Errorf("expected host1 allowed: %v", err)
	}
	if err := f.AllowHost("host3"); err == nil {
		t.Error("expected host3 denied when not in allowlist")
	}
}

func TestFilter_AllowHost_Denylist(t *testing.T) {
	f, err := NewFilter(nil, []string{"bad-host"}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := f.AllowHost("good-host"); err != nil {
		t.Errorf("expected good-host allowed: %v", err)
	}
	if err := f.AllowHost("bad-host"); err == nil {
		t.Error("expected bad-host denied")
	}
}

func TestFilter_AllowHost_DenylistPriority(t *testing.T) {
	f, err := NewFilter([]string{"host1"}, []string{"host1"}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Denylist should take priority over allowlist.
	if err := f.AllowHost("host1"); err == nil {
		t.Error("expected host1 denied when in both allow and deny lists")
	}
}

func TestFilter_AllowHost_Regex(t *testing.T) {
	f, err := NewFilter([]string{`192\.168\..*`}, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := f.AllowHost("192.168.1.1"); err != nil {
		t.Errorf("expected 192.168.1.1 allowed: %v", err)
	}
	if err := f.AllowHost("10.0.0.1"); err == nil {
		t.Error("expected 10.0.0.1 denied")
	}
}

func TestFilter_PartialMatchPrevented(t *testing.T) {
	// Denylist "rm" should NOT match "format" due to auto-anchoring.
	f, err := NewFilter(nil, nil, nil, []string{`rm`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := f.AllowCommand("format"); err != nil {
		t.Errorf("expected 'format' allowed (not partial match on 'rm'): %v", err)
	}
	if err := f.AllowCommand("rm"); err == nil {
		t.Error("expected exact 'rm' denied")
	}
}

func TestFilter_AutoAnchor_Wildcard(t *testing.T) {
	// Patterns with .* should still work for substring matching.
	f, err := NewFilter(nil, nil, nil, []string{`rm\s+-rf.*`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := f.AllowCommand("rm -rf /"); err == nil {
		t.Error("expected 'rm -rf /' denied")
	}
	if err := f.AllowCommand("ls -la"); err != nil {
		t.Errorf("expected 'ls -la' allowed: %v", err)
	}
}

func TestFilter_AllowCommand_EmptyLists(t *testing.T) {
	f, err := NewFilter(nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := f.AllowCommand("any command"); err != nil {
		t.Errorf("expected allow all when lists are empty: %v", err)
	}
}

func TestFilter_AllowCommand_Denylist(t *testing.T) {
	f, err := NewFilter(nil, nil, nil, []string{`rm\s+-rf.*`, `shutdown.*`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := f.AllowCommand("ls -la"); err != nil {
		t.Errorf("expected ls allowed: %v", err)
	}
	if err := f.AllowCommand("rm -rf /"); err == nil {
		t.Error("expected rm -rf denied")
	}
	if err := f.AllowCommand("shutdown now"); err == nil {
		t.Error("expected shutdown denied")
	}
}

func TestFilter_AllowCommand_Allowlist(t *testing.T) {
	f, err := NewFilter(nil, nil, []string{`^ls.*`, `^cat.*`}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := f.AllowCommand("ls -la"); err != nil {
		t.Errorf("expected ls allowed: %v", err)
	}
	if err := f.AllowCommand("cat /etc/passwd"); err != nil {
		t.Errorf("expected cat allowed: %v", err)
	}
	if err := f.AllowCommand("rm -rf /"); err == nil {
		t.Error("expected rm denied when not in allowlist")
	}
}

func TestFilter_InvalidRegex(t *testing.T) {
	_, err := NewFilter(nil, []string{"[invalid"}, nil, nil)
	if err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestFilter_CIDR_Allowlist(t *testing.T) {
	f, err := NewFilter([]string{"10.0.0.0/8"}, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := f.AllowHost("10.1.2.3"); err != nil {
		t.Errorf("expected 10.1.2.3 allowed within 10.0.0.0/8: %v", err)
	}
	if err := f.AllowHost("10.255.255.255"); err != nil {
		t.Errorf("expected 10.255.255.255 allowed within 10.0.0.0/8: %v", err)
	}
	if err := f.AllowHost("192.168.1.1"); err == nil {
		t.Error("expected 192.168.1.1 denied (not in 10.0.0.0/8)")
	}
}

func TestFilter_CIDR_Denylist(t *testing.T) {
	f, err := NewFilter(nil, []string{"192.168.0.0/16"}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := f.AllowHost("192.168.1.1"); err == nil {
		t.Error("expected 192.168.1.1 denied by 192.168.0.0/16")
	}
	if err := f.AllowHost("10.0.0.1"); err != nil {
		t.Errorf("expected 10.0.0.1 allowed (not in 192.168.0.0/16): %v", err)
	}
}

func TestFilter_CIDR_DenylistPriority(t *testing.T) {
	f, err := NewFilter([]string{"10.0.0.0/8"}, []string{"10.0.1.0/24"}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 10.0.1.5 is in both allow (10.0.0.0/8) and deny (10.0.1.0/24) — deny wins.
	if err := f.AllowHost("10.0.1.5"); err == nil {
		t.Error("expected 10.0.1.5 denied (denylist priority)")
	}
	// 10.0.2.5 is in allow but not deny — allowed.
	if err := f.AllowHost("10.0.2.5"); err != nil {
		t.Errorf("expected 10.0.2.5 allowed: %v", err)
	}
}

func TestFilter_CIDR_MixedWithRegex(t *testing.T) {
	f, err := NewFilter([]string{"10.0.0.0/8", `my-server\.example\.com`}, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := f.AllowHost("10.5.5.5"); err != nil {
		t.Errorf("expected 10.5.5.5 allowed by CIDR: %v", err)
	}
	if err := f.AllowHost("my-server.example.com"); err != nil {
		t.Errorf("expected my-server.example.com allowed by regex: %v", err)
	}
	if err := f.AllowHost("other.example.com"); err == nil {
		t.Error("expected other.example.com denied")
	}
}

func TestFilter_CIDR_InvalidFallsToRegex(t *testing.T) {
	// "192.168/notcidr" is not valid CIDR, should fall through to regex.
	// As a regex it will also fail to match.
	f, err := NewFilter([]string{`host\.with/slash`}, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := f.AllowHost("host.with/slash"); err != nil {
		t.Errorf("expected host.with/slash allowed by regex fallback: %v", err)
	}
}

func TestFilter_CIDR_IPv6(t *testing.T) {
	f, err := NewFilter([]string{"fd00::/8"}, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := f.AllowHost("fd00::1"); err != nil {
		t.Errorf("expected fd00::1 allowed within fd00::/8: %v", err)
	}
	if err := f.AllowHost("2001:db8::1"); err == nil {
		t.Error("expected 2001:db8::1 denied (not in fd00::/8)")
	}
}

func TestFilter_CIDR_HostnameNotMatchedByCIDR(t *testing.T) {
	// CIDR matchers only match IP addresses, not hostnames.
	f, err := NewFilter([]string{"10.0.0.0/8"}, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := f.AllowHost("my-hostname"); err == nil {
		t.Error("expected hostname denied (CIDR only matches IPs)")
	}
}
