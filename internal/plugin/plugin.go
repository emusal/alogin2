// Package plugin implements the Application-Aware Plugin System for alogin.
// Plugins are defined as YAML files in ~/.config/alogin/plugins/*.yaml and
// describe how to launch an application (DB client, container shell, etc.)
// on a remote host, including credential injection and runtime detection.
package plugin

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ExposeMode controls how a resolved secret is delivered to the application.
type ExposeMode string

const (
	// ExposeModePrompt injects via PTY Expect-Send (interactive prompt automation).
	ExposeModePrompt ExposeMode = "prompt"
	// ExposeModeEnv injects the secret as an SSH session environment variable.
	ExposeModeEnv ExposeMode = "env"
	// ExposeModeArg substitutes the secret directly into the command arguments.
	ExposeModeArg ExposeMode = "arg"
	// ExposeModeFile writes the secret to a temp file and injects the path.
	ExposeModeFile ExposeMode = "file"
)

// AuthProvider selects the source backend for fetching secrets.
type AuthProvider string

const (
	AuthProviderVault  AuthProvider = "vault"
	AuthProviderEnv    AuthProvider = "env"
	AuthProviderStatic AuthProvider = "static"
)

// AutomationSpec describes the Expect-Send rule for one prompt-mode secret.
type AutomationSpec struct {
	Expect      string `yaml:"expect"`       // substring to match in PTY output
	Send        string `yaml:"send"`         // value to send; may use {{VAR_NAME}} template
	SendNewline bool   `yaml:"send_newline"` // append \n after Send
}

// VarMapping defines one secret variable: where to fetch it and how to inject it.
type VarMapping struct {
	Var        string          `yaml:"var"`                  // variable name, e.g. "DB_PASS"
	Path       string          `yaml:"path"`                 // vault account key, env var name, or static value
	Expose     ExposeMode      `yaml:"expose"`               // injection method
	Automation *AutomationSpec `yaml:"automation,omitempty"` // required when Expose == prompt
}

// AuthSpec holds credential configuration for a plugin.
type AuthSpec struct {
	Provider AuthProvider `yaml:"provider"` // vault | env | static
	Mapping  []VarMapping `yaml:"mapping"`
}

// NativeEnv describes execution in the host's native environment.
type NativeEnv struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
}

// DockerEnv describes execution inside a Docker container.
type DockerEnv struct {
	ContainerMatch string   `yaml:"container_match"` // substring matched against `docker ps` output
	Command        string   `yaml:"command"`
	Args           []string `yaml:"args"`
}

// RuntimeEnvironments holds strategy-specific execution configurations.
type RuntimeEnvironments struct {
	Native *NativeEnv `yaml:"native,omitempty"`
	Docker *DockerEnv `yaml:"docker,omitempty"`
}

// RuntimeSpec describes how the application is launched on the remote host.
type RuntimeSpec struct {
	Strategies   []string            `yaml:"strategies"`    // priority order, e.g. ["docker", "native"]
	Environments RuntimeEnvironments `yaml:"environments"`
	CmdFlag      string              `yaml:"cmd_flag"`      // flag used to pass --cmd value, e.g. "-e" or "--eval" (default: "-e")
}

// Plugin is the fully parsed and validated plugin definition.
type Plugin struct {
	Name    string      `yaml:"name"`
	Version string      `yaml:"version"`
	Auth    AuthSpec    `yaml:"auth"`
	Runtime RuntimeSpec `yaml:"runtime"`

	// FilePath is the absolute path of the YAML file this plugin was loaded from.
	// Not part of the YAML schema — set by LoadFromFile/LoadDir.
	FilePath string `yaml:"-"`
}

// PluginDir returns the conventional plugins directory under configDir.
func PluginDir(configDir string) string {
	return filepath.Join(configDir, "plugins")
}

// LoadFromFile parses and validates a single plugin YAML file.
func LoadFromFile(path string) (*Plugin, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read plugin %s: %w", path, err)
	}
	var p Plugin
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse plugin %s: %w", path, err)
	}
	if err := validate(&p); err != nil {
		return nil, fmt.Errorf("invalid plugin %s: %w", path, err)
	}
	p.FilePath = path
	return &p, nil
}

// LoadDir loads all *.yaml files from dir. Files that fail to parse are skipped.
func LoadDir(dir string) ([]*Plugin, error) {
	entries, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	var plugins []*Plugin
	for _, path := range entries {
		p, err := LoadFromFile(path)
		if err != nil {
			continue
		}
		plugins = append(plugins, p)
	}
	return plugins, nil
}

func validate(p *Plugin) error {
	if p.Name == "" {
		return fmt.Errorf("name is required")
	}
	if len(p.Runtime.Strategies) == 0 {
		return fmt.Errorf("runtime.strategies must have at least one entry")
	}
	for _, m := range p.Auth.Mapping {
		if m.Expose == ExposeModePrompt && m.Automation == nil {
			return fmt.Errorf("var %q: automation is required when expose is prompt", m.Var)
		}
	}
	return nil
}
