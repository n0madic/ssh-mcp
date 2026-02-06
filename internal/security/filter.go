package security

import (
	"fmt"
	"regexp"
	"strings"
)

// Filter provides host and command allowlist/denylist checking.
type Filter struct {
	hostAllowlist []*regexp.Regexp
	hostDenylist  []*regexp.Regexp
	cmdAllowlist  []*regexp.Regexp
	cmdDenylist   []*regexp.Regexp
}

// NewFilter creates a new Filter from string patterns.
func NewFilter(hostAllow, hostDeny, cmdAllow, cmdDeny []string) (*Filter, error) {
	f := &Filter{}
	var err error

	if f.hostAllowlist, err = compilePatterns(hostAllow); err != nil {
		return nil, fmt.Errorf("host allowlist: %w", err)
	}
	if f.hostDenylist, err = compilePatterns(hostDeny); err != nil {
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

	for _, re := range f.hostDenylist {
		if re.MatchString(host) {
			return fmt.Errorf("host %q is denied by denylist pattern %q", host, re.String())
		}
	}

	if len(f.hostAllowlist) > 0 {
		for _, re := range f.hostAllowlist {
			if re.MatchString(host) {
				return nil
			}
		}
		return fmt.Errorf("host %q is not in the allowlist", host)
	}

	return nil
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
		// Auto-anchor patterns for full-string matching to prevent
		// partial matches (e.g. denylist "rm" matching "format").
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
		compiled = append(compiled, re)
	}
	return compiled, nil
}
