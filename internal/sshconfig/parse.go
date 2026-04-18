// Package sshconfig parses OpenSSH config files and imports them into alogin.
package sshconfig

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"
)

// HostBlock holds all parsed directives for a single Host stanza.
type HostBlock struct {
	Pattern      string // raw Host keyword value
	HostName     string // HostName directive; falls back to Pattern if absent
	User         string
	Port         int    // 0 = use SSH default (22)
	IdentityFile string // first IdentityFile directive, ~ expanded
	ProxyJump    string // raw ProxyJump value
	ProxyCommand string // raw ProxyCommand value
	IsWildcard   bool   // true when Pattern contains *, ?, or [
	// Optional cipher/kex/hostkey overrides (empty = use library defaults).
	// KexAlgorithms: a leading "+" in the ssh_config value is stripped — the
	// parsed list replaces the default entirely when stored in alogin.
	Ciphers           []string
	KexAlgorithms     []string
	HostKeyAlgorithms []string
}

// ParsedConfig is the result of parsing one SSH config file.
type ParsedConfig struct {
	Blocks []*HostBlock
}

// JumpHop represents one resolved hop from a ProxyJump chain or ProxyCommand.
type JumpHop struct {
	Host string
	User string // empty = inherit
	Port int    // 0 = default 22
}

// ParseFile reads and parses an SSH config file at path.
func ParseFile(path string) (*ParsedConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f)
}

// Parse reads SSH config directives from r and returns the parsed config.
func Parse(r io.Reader) (*ParsedConfig, error) {
	home, _ := os.UserHomeDir()

	var blocks []*HostBlock
	var cur *HostBlock

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, val := splitKeyValue(line)
		if key == "" {
			continue
		}
		key = strings.ToLower(key)

		switch key {
		case "host":
			if cur != nil {
				blocks = append(blocks, cur)
			}
			cur = &HostBlock{
				Pattern:    val,
				HostName:   val, // default until HostName directive overrides
				IsWildcard: containsGlob(val),
			}
		case "hostname":
			if cur != nil {
				cur.HostName = val
			}
		case "user":
			if cur != nil {
				cur.User = val
			}
		case "port":
			if cur != nil {
				cur.Port, _ = strconv.Atoi(val)
			}
		case "identityfile":
			if cur != nil && cur.IdentityFile == "" {
				cur.IdentityFile = expandTilde(val, home)
			}
		case "proxyjump":
			if cur != nil && cur.ProxyJump == "" {
				cur.ProxyJump = val
			}
		case "proxycommand":
			if cur != nil && cur.ProxyCommand == "" {
				cur.ProxyCommand = val
			}
		case "ciphers":
			if cur != nil && len(cur.Ciphers) == 0 {
				cur.Ciphers = splitAlgoList(val)
			}
		case "hostkeyalgorithms":
			if cur != nil && len(cur.HostKeyAlgorithms) == 0 {
				cur.HostKeyAlgorithms = splitAlgoList(val)
			}
		case "kexalgorithms":
			if cur != nil && len(cur.KexAlgorithms) == 0 {
				// Strip leading "+" (append-to-defaults syntax); we store the
				// explicit list and replace defaults entirely on connect.
				cur.KexAlgorithms = splitAlgoList(strings.TrimPrefix(val, "+"))
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if cur != nil {
		blocks = append(blocks, cur)
	}
	return &ParsedConfig{Blocks: blocks}, nil
}

// ParseProxyJump parses a ProxyJump directive value into an ordered hop chain.
// Returns nil for "none" or empty input.
func ParseProxyJump(value string) []JumpHop {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "none") {
		return nil
	}
	parts := strings.Split(value, ",")
	hops := make([]JumpHop, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		hop := parseJumpHopString(p)
		hops = append(hops, hop)
	}
	return hops
}

// parseJumpHopString parses a single hop string like "user@host:port".
func parseJumpHopString(s string) JumpHop {
	var hop JumpHop
	// split user@ prefix
	if at := strings.LastIndex(s, "@"); at >= 0 {
		hop.User = s[:at]
		s = s[at+1:]
	}
	// split :port suffix
	if colon := strings.LastIndex(s, ":"); colon >= 0 {
		portStr := s[colon+1:]
		if p, err := strconv.Atoi(portStr); err == nil {
			hop.Port = p
			s = s[:colon]
		}
	}
	hop.Host = s
	return hop
}

// ParseProxyCommand attempts a best-effort extraction of the jump hostname from
// a ProxyCommand directive. Returns (JumpHop{}, false) when extraction fails.
//
// Recognised patterns:
//
//	ssh [-flags] <host> [-W %h:%p]
//	ssh [-flags] -W %h:%p <host>
//	ssh [-flags] <host> [nc %h %p]
func ParseProxyCommand(value string) (JumpHop, bool) {
	tokens := strings.Fields(value)
	if len(tokens) == 0 || strings.ToLower(tokens[0]) != "ssh" {
		return JumpHop{}, false
	}

	// flags that consume the next token as their argument
	argFlags := map[string]bool{
		"-W": true, "-o": true, "-i": true, "-F": true,
		"-J": true, "-E": true, "-S": true, "-b": true,
		"-c": true, "-D": true, "-e": true, "-I": true,
		"-L": true, "-m": true, "-O": true, "-Q": true,
		"-R": true, "-w": true,
	}
	// boolean flags
	boolFlags := map[string]bool{
		"-q": true, "-A": true, "-a": true, "-x": true, "-X": true,
		"-T": true, "-v": true, "-C": true, "-n": true, "-N": true,
		"-f": true, "-4": true, "-6": true, "-g": true, "-k": true,
		"-M": true, "-s": true, "-t": true, "-Y": true,
	}
	skipNext := map[string]bool{"%h": true, "%p": true, "%r": true, "nc": true}

	var hop JumpHop
	i := 1
	for i < len(tokens) {
		t := tokens[i]
		if argFlags[t] {
			if t == "-p" && i+1 < len(tokens) {
				hop.Port, _ = strconv.Atoi(tokens[i+1])
			} else if t == "-l" && i+1 < len(tokens) {
				hop.User = tokens[i+1]
			}
			i += 2
			continue
		}
		// -p and -l are arg flags not in the map above; handle combined form
		if t == "-p" || t == "-l" {
			if i+1 < len(tokens) {
				if t == "-p" {
					hop.Port, _ = strconv.Atoi(tokens[i+1])
				} else {
					hop.User = tokens[i+1]
				}
			}
			i += 2
			continue
		}
		if boolFlags[t] || strings.HasPrefix(t, "-vv") {
			i++
			continue
		}
		if skipNext[t] {
			i++
			continue
		}
		if !strings.HasPrefix(t, "-") && hop.Host == "" {
			hop.Host = t
		}
		i++
	}
	if hop.Host == "" || hop.Host == "%h" {
		return JumpHop{}, false
	}
	return hop, true
}

// splitKeyValue splits an SSH config line into key and value.
// Handles both "Key Value" and "Key=Value" forms.
func splitKeyValue(line string) (key, value string) {
	// try = separator first
	if idx := strings.IndexByte(line, '='); idx >= 0 {
		k := strings.TrimSpace(line[:idx])
		v := strings.TrimSpace(line[idx+1:])
		if !strings.ContainsAny(k, " \t") {
			return k, trimQuotes(v)
		}
	}
	// fall back to whitespace separator
	parts := strings.SplitN(line, " ", 2)
	if len(parts) == 1 {
		parts = strings.SplitN(line, "\t", 2)
	}
	if len(parts) < 2 {
		return strings.TrimSpace(parts[0]), ""
	}
	return strings.TrimSpace(parts[0]), trimQuotes(strings.TrimSpace(parts[1]))
}

func trimQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func containsGlob(s string) bool {
	return strings.ContainsAny(s, "*?[") || strings.Contains(s, " ")
}

// splitAlgoList splits a comma-separated algorithm list (e.g. Ciphers value)
// and trims whitespace from each element.
func splitAlgoList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func expandTilde(path, home string) string {
	if strings.HasPrefix(path, "~/") && home != "" {
		return home + path[1:]
	}
	if path == "~" && home != "" {
		return home
	}
	return path
}
