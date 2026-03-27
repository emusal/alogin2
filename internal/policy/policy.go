// Package policy implements the agent-policy.yaml RBAC engine for alogin's MCP server.
// It evaluates whether a given agent request should be allowed, denied, or held for
// human-in-the-loop (HITL) approval.
package policy

import (
	"errors"
	"fmt"
	"os"
	"path"
	"regexp"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// PolicyFile is the top-level structure of agent-policy.yaml.
type PolicyFile struct {
	Version        int    `yaml:"version"`
	DefaultAction  string `yaml:"default_action"`   // "allow" | "deny" | "require_approval"
	HITLTimeoutSec int    `yaml:"hitl_timeout_sec"` // 0 = use default (120)
	Rules          []Rule `yaml:"rules"`
}

// Rule is a single named policy rule.
type Rule struct {
	Name   string    `yaml:"name"`
	Match  MatchSpec `yaml:"match"`
	Action string    `yaml:"action"` // "allow" | "deny" | "require_approval"
}

// MatchSpec describes what conditions trigger a rule.
// All specified conditions must match (logical AND). Unset fields are wildcards.
type MatchSpec struct {
	Commands   []string `yaml:"commands"`    // regex patterns; any command matching any pattern fires this rule
	AgentIDs   []string `yaml:"agent_id"`    // glob patterns (path.Match); empty = any agent
	ServerIDs  []int64  `yaml:"server_ids"`  // exact server_id; empty = any server
	ClusterIDs []int64  `yaml:"cluster_ids"` // exact cluster_id; empty = any cluster
	TimeWindow string   `yaml:"time_window"` // "HH:MM-HH:MM" UTC; empty = any time
}

// CheckRequest is the input to Engine.Check.
type CheckRequest struct {
	AgentID   string
	Commands  []string
	ServerID  int64 // 0 if cluster exec
	ClusterID int64 // 0 if single-server exec
}

// CheckResult is the decision from Engine.Check.
type CheckResult struct {
	Action   string // "allow", "deny", or "require_approval"
	RuleName string // name of the rule that matched; "" = default_action applied
}

// Engine holds compiled policy rules ready for fast evaluation.
type Engine struct {
	file             PolicyFile
	compiledCommands [][]*regexp.Regexp // [ruleIndex][patternIndex]
}

// LoadFile reads and parses the policy YAML file at path.
// Returns (nil, nil) if the file does not exist — a nil Engine means allow-all
// with built-in destructive-command checks only.
func LoadFile(filePath string) (*Engine, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read policy file: %w", err)
	}
	var pf PolicyFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("parse policy file: %w", err)
	}
	return New(pf)
}

// New constructs an Engine from a PolicyFile, compiling all command regexps.
func New(pf PolicyFile) (*Engine, error) {
	if pf.DefaultAction == "" {
		pf.DefaultAction = "allow"
	}
	compiled := make([][]*regexp.Regexp, len(pf.Rules))
	for i, rule := range pf.Rules {
		for _, pat := range rule.Match.Commands {
			re, err := regexp.Compile(pat)
			if err != nil {
				return nil, fmt.Errorf("policy rule %q: invalid command pattern %q: %w", rule.Name, pat, err)
			}
			compiled[i] = append(compiled[i], re)
		}
	}
	return &Engine{file: pf, compiledCommands: compiled}, nil
}

// Check evaluates rules top-to-bottom and returns the first match.
// If no rule matches, the default_action is returned.
func (e *Engine) Check(req CheckRequest) CheckResult {
	for i, rule := range e.file.Rules {
		if matchesRule(rule, e.compiledCommands[i], req) {
			return CheckResult{Action: rule.Action, RuleName: rule.Name}
		}
	}
	return CheckResult{Action: e.file.DefaultAction}
}

// HITLTimeout returns the configured HITL approval timeout.
func (e *Engine) HITLTimeout() time.Duration {
	if e.file.HITLTimeoutSec > 0 {
		return time.Duration(e.file.HITLTimeoutSec) * time.Second
	}
	return 120 * time.Second
}

// matchesRule returns true when all specified conditions in the rule's MatchSpec are satisfied.
func matchesRule(rule Rule, compiled []*regexp.Regexp, req CheckRequest) bool {
	m := rule.Match

	// Commands: any command must match at least one compiled pattern.
	if len(compiled) > 0 {
		if !anyCommandMatchesAnyPattern(req.Commands, compiled) {
			return false
		}
	}

	// AgentIDs: glob match.
	if len(m.AgentIDs) > 0 {
		if !matchesGlob(m.AgentIDs, req.AgentID) {
			return false
		}
	}

	// ServerIDs: exact match.
	if len(m.ServerIDs) > 0 && req.ServerID != 0 {
		if !containsInt64(m.ServerIDs, req.ServerID) {
			return false
		}
	}

	// ClusterIDs: exact match.
	if len(m.ClusterIDs) > 0 && req.ClusterID != 0 {
		if !containsInt64(m.ClusterIDs, req.ClusterID) {
			return false
		}
	}

	// TimeWindow: current UTC time must fall within "HH:MM-HH:MM".
	if m.TimeWindow != "" {
		if !matchesTimeWindow(m.TimeWindow) {
			return false
		}
	}

	return true
}

func anyCommandMatchesAnyPattern(commands []string, patterns []*regexp.Regexp) bool {
	for _, cmd := range commands {
		for _, re := range patterns {
			if re.MatchString(cmd) {
				return true
			}
		}
	}
	return false
}

func matchesGlob(patterns []string, s string) bool {
	for _, pat := range patterns {
		matched, err := path.Match(pat, s)
		if err == nil && matched {
			return true
		}
	}
	return false
}

func containsInt64(slice []int64, v int64) bool {
	for _, x := range slice {
		if x == v {
			return true
		}
	}
	return false
}

// matchesTimeWindow returns true if the current UTC time is within "HH:MM-HH:MM".
func matchesTimeWindow(window string) bool {
	if len(window) != 11 || window[5] != '-' {
		return false
	}
	startH, startM, ok1 := parseHHMM(window[:5])
	endH, endM, ok2 := parseHHMM(window[6:])
	if !ok1 || !ok2 {
		return false
	}
	now := time.Now().UTC()
	nowMins := now.Hour()*60 + now.Minute()
	startMins := startH*60 + startM
	endMins := endH*60 + endM
	if startMins <= endMins {
		return nowMins >= startMins && nowMins < endMins
	}
	// Wraps midnight
	return nowMins >= startMins || nowMins < endMins
}

func parseHHMM(s string) (h, m int, ok bool) {
	if len(s) != 5 || s[2] != ':' {
		return
	}
	h = int(s[0]-'0')*10 + int(s[1]-'0')
	m = int(s[3]-'0')*10 + int(s[4]-'0')
	if h > 23 || m > 59 {
		return
	}
	ok = true
	return
}

// ResolveFor returns the effective policy engine for a given server.
// If policyYAML is empty (server has no per-server override), the global engine is returned.
// If policyYAML is non-empty it is parsed and compiled into a new Engine.
// A parse error is returned explicitly — a malformed per-server policy should not silently
// fall back to the global engine.
func ResolveFor(global *Engine, policyYAML string) (*Engine, error) {
	if policyYAML == "" {
		return global, nil
	}
	var pf PolicyFile
	if err := yaml.Unmarshal([]byte(policyYAML), &pf); err != nil {
		return nil, fmt.Errorf("parse per-server policy: %w", err)
	}
	return New(pf)
}

// --- Built-in destructive pattern check (used when Engine is nil) ---

// DefaultDestructivePatterns is the built-in list of patterns that trigger
// require_approval even when no agent-policy.yaml is present.
var DefaultDestructivePatterns = []string{
	`^rm\s`,
	`^rm$`,
	`\brm\s+(-[rRfFi]*[rR][fFi]*|-[fFi]*[fF][rRi]*)\b`,
	`^dd\s`,
	`^mkfs`,
	`^shutdown`,
	`^reboot`,
	`^halt`,
	`^poweroff`,
	`(^systemctl|^service)\s+(stop|disable|mask)\b`,
	`(?i)(^drop|^truncate)\s+.*table\b`,
	`^>\s`,
}

var (
	destructiveOnce     sync.Once
	destructiveCompiled []*regexp.Regexp
)

func getDestructivePatterns() []*regexp.Regexp {
	destructiveOnce.Do(func() {
		for _, pat := range DefaultDestructivePatterns {
			re, err := regexp.Compile(pat)
			if err == nil {
				destructiveCompiled = append(destructiveCompiled, re)
			}
		}
	})
	return destructiveCompiled
}

// IsDestructive returns true if any of the given commands matches a built-in
// destructive pattern. Used when no Engine (no policy file) is loaded.
func IsDestructive(commands []string) bool {
	return anyCommandMatchesAnyPattern(commands, getDestructivePatterns())
}
