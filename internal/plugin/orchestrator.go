package plugin

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/emusal/alogin2/internal/vault"
)

// Session holds resolved state for one plugin execution: secrets, strategy,
// and the original plugin definition.
type Session struct {
	Plugin    *Plugin
	Strategy  *Strategy
	Secrets   Secrets
	ExtraArgs []string // additional args appended to the command (e.g. from --cmd)
}

// Prepare resolves secrets and detects the runtime strategy.
// This is the setup phase; no interactive execution happens here.
func Prepare(ctx context.Context, p *Plugin, vlt vault.Vault, runner RemoteRunner) (*Session, error) {
	secrets, err := ResolveSecrets(p, vlt)
	if err != nil {
		return nil, fmt.Errorf("resolve secrets: %w", err)
	}

	strategy, err := DetectStrategy(ctx, p, runner)
	if err != nil {
		return nil, err
	}

	// Expand {{VAR}} placeholders in strategy args (expose: arg).
	expanded := make([]string, len(strategy.Args))
	for i, a := range strategy.Args {
		expanded[i] = ApplyTemplate(a, secrets)
	}
	strategy.Args = expanded

	return &Session{
		Plugin:   p,
		Strategy: strategy,
		Secrets:  secrets,
	}, nil
}

// BuildPTYRules converts prompt-expose mappings into PTYRules with
// secrets already substituted. The resulting rules are passed to RunPTY.
func (s *Session) BuildPTYRules() []PTYRule {
	var rules []PTYRule
	for _, m := range s.Plugin.Auth.Mapping {
		if m.Expose != ExposeModePrompt || m.Automation == nil {
			continue
		}
		expanded := ApplyTemplate(m.Automation.Send, s.Secrets)
		rules = append(rules, PTYRule{
			Pattern:        m.Automation.Expect,
			Send:           expanded,
			SendNewline:    m.Automation.SendNewline,
			EchoSuppressed: true, // passwords are always sent without echo
		})
	}
	return rules
}

// EnvVars returns a map of environment variables to inject for env-expose vars.
func (s *Session) EnvVars() map[string]string {
	env := make(map[string]string)
	for _, m := range s.Plugin.Auth.Mapping {
		if m.Expose == ExposeModeEnv {
			env[m.Var] = s.Secrets[m.Var]
		}
	}
	return env
}

// AuditVars returns the names of injected variables for audit logging.
// Values are never included.
func (s *Session) AuditVars() []string {
	names := make([]string, 0, len(s.Secrets))
	for k := range s.Secrets {
		names = append(names, k)
	}
	return names
}

// Launch starts the application on the remote host.
//
// Interactive mode (no ExtraArgs): attaches the current terminal directly so
// the user gets a live prompt (mysql>, psql#, etc.).
//
// Non-interactive mode (ExtraArgs set via --cmd): uses RunPTY for prompt-expose
// mappings, or Run for env/arg mappings.
func (s *Session) Launch(ctx context.Context, runner RemoteRunner) (string, error) {
	cmd := s.Strategy.BuildCommand(s.ExtraArgs...)
	rules := s.BuildPTYRules()

	// Interactive: apply PTY rules first (e.g. password prompt), then hand
	// the terminal to the user for the rest of the session.
	if len(s.ExtraArgs) == 0 {
		return "", runner.RunInteractive(ctx, cmd, rules, s.EnvVars())
	}

	// Prepend VAR=val pairs for env-expose mappings.
	if envVars := s.EnvVars(); len(envVars) > 0 {
		keys := make([]string, 0, len(envVars))
		for k := range envVars {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		prefix := make([]string, 0, len(keys))
		for _, k := range keys {
			prefix = append(prefix, k+"="+envVars[k])
		}
		cmd = strings.Join(prefix, " ") + " " + cmd
	}

	if len(rules) > 0 {
		return runner.RunPTY(ctx, cmd, rules)
	}
	out, _, err := runner.Run(ctx, cmd)
	return out, err
}
