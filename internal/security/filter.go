package security

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

// hostMatcher is an interface for matching hosts by regex or CIDR.
type hostMatcher interface {
	match(host string) bool
	String() string
}

// regexMatcher matches hosts using a compiled regex.
type regexMatcher struct {
	re *regexp.Regexp
}

func (m *regexMatcher) match(host string) bool {
	return m.re.MatchString(host)
}

func (m *regexMatcher) String() string {
	return m.re.String()
}

// cidrMatcher matches hosts by checking if their IP falls within a CIDR range.
type cidrMatcher struct {
	ipNet *net.IPNet
	cidr  string
}

func (m *cidrMatcher) match(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return m.ipNet.Contains(ip)
}

func (m *cidrMatcher) String() string {
	return m.cidr
}

// Filter provides host and command allowlist/denylist checking.
type Filter struct {
	hostAllowlist []hostMatcher
	hostDenylist  []hostMatcher
	cmdAllowlist  []*regexp.Regexp
	cmdDenylist   []*regexp.Regexp
}

// NewFilter creates a new Filter from string patterns.
func NewFilter(hostAllow, hostDeny, cmdAllow, cmdDeny []string) (*Filter, error) {
	f := &Filter{}
	var err error

	if f.hostAllowlist, err = compileHostPatterns(hostAllow); err != nil {
		return nil, fmt.Errorf("host allowlist: %w", err)
	}
	if f.hostDenylist, err = compileHostPatterns(hostDeny); err != nil {
		return nil, fmt.Errorf("host denylist: %w", err)
	}
	if f.cmdAllowlist, err = compilePatterns(cmdAllow); err != nil {
		return nil, fmt.Errorf("command allowlist: %w", err)
	}
	if f.cmdDenylist, err = compilePatterns(cmdDeny); err != nil {
		return nil, fmt.Errorf("command denylist: %w", err)
	}

	return f, nil
}

// AllowHost checks if a host is allowed.
// Denylist has priority; empty allowlist means allow all.
func (f *Filter) AllowHost(host string) error {
	host = strings.ToLower(host)

	for _, m := range f.hostDenylist {
		if m.match(host) {
			return fmt.Errorf("host %q is denied by denylist pattern %q", host, m.String())
		}
	}

	if len(f.hostAllowlist) > 0 {
		for _, m := range f.hostAllowlist {
			if m.match(host) {
				return nil
			}
		}
		return fmt.Errorf("host %q is not in the allowlist", host)
	}

	return nil
}

// compileHostPatterns compiles host patterns as either CIDR matchers or regex matchers.
func compileHostPatterns(patterns []string) ([]hostMatcher, error) {
	matchers := make([]hostMatcher, 0, len(patterns))
	for _, p := range patterns {
		// Try CIDR first: pattern must contain "/" and parse successfully.
		if strings.Contains(p, "/") {
			_, ipNet, err := net.ParseCIDR(p)
			if err == nil {
				matchers = append(matchers, &cidrMatcher{ipNet: ipNet, cidr: p})
				continue
			}
		}
		// Fall through to regex.
		re, err := compileAnchoredRegex(p)
		if err != nil {
			return nil, err
		}
		matchers = append(matchers, &regexMatcher{re: re})
	}
	return matchers, nil
}

// AllowCommand checks if a command is allowed.
// Denylist has priority; empty allowlist means allow all.
func (f *Filter) AllowCommand(cmd string) error {
	for _, re := range f.cmdDenylist {
		if re.MatchString(cmd) {
			return fmt.Errorf("command is denied by denylist pattern %q", re.String())
		}
	}

	if len(f.cmdAllowlist) > 0 {
		for _, re := range f.cmdAllowlist {
			if re.MatchString(cmd) {
				return nil
			}
		}
		return fmt.Errorf("command is not in the allowlist")
	}

	return nil
}

func compilePatterns(patterns []string) ([]*regexp.Regexp, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := compileAnchoredRegex(p)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, re)
	}
	return compiled, nil
}

// compileAnchoredRegex compiles a regex pattern with auto-anchoring for full-string matching.
func compileAnchoredRegex(p string) (*regexp.Regexp, error) {
	anchored := p
	if !strings.HasPrefix(anchored, "^") {
		anchored = "^" + anchored
	}
	if !strings.HasSuffix(anchored, "$") {
		anchored = anchored + "$"
	}
	re, err := regexp.Compile(anchored)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern %q: %w", p, err)
	}
	return re, nil
}
