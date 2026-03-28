package plugin

import (
	"context"
	"fmt"
	"strings"
)

// Strategy is a resolved execution strategy for a plugin session.
type Strategy struct {
	Kind        string   // "docker" | "native"
	ContainerID string   // Docker only: matched container ID
	Command     string   // executable name
	Args        []string // command arguments
}

// DetectStrategy probes the remote host and returns the first viable Strategy
// according to the plugin's strategy priority list.
func DetectStrategy(ctx context.Context, p *Plugin, runner RemoteRunner) (*Strategy, error) {
	for _, kind := range p.Runtime.Strategies {
		switch kind {
		case "docker":
			if s, ok := detectDocker(ctx, p, runner); ok {
				return s, nil
			}
		case "native":
			if s, ok := detectNative(ctx, p, runner); ok {
				return s, nil
			}
		}
	}
	return nil, fmt.Errorf("no viable runtime found for plugin %q (tried: %v)",
		p.Name, p.Runtime.Strategies)
}

func detectDocker(ctx context.Context, p *Plugin, runner RemoteRunner) (*Strategy, bool) {
	docker := p.Runtime.Environments.Docker
	if docker == nil {
		return nil, false
	}
	out, code, err := runner.Run(ctx, "docker ps --format '{{.ID}} {{.Image}} {{.Names}}'")
	if err != nil || code != 0 {
		return nil, false
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		if strings.Contains(strings.ToLower(line), strings.ToLower(docker.ContainerMatch)) {
			containerID := ""
			if fields := strings.Fields(line); len(fields) > 0 {
				containerID = fields[0]
			}
			return &Strategy{
				Kind:        "docker",
				ContainerID: containerID,
				Command:     docker.Command,
				Args:        docker.Args,
			}, true
		}
	}
	return nil, false
}

func detectNative(ctx context.Context, p *Plugin, runner RemoteRunner) (*Strategy, bool) {
	native := p.Runtime.Environments.Native
	if native == nil {
		return nil, false
	}
	_, code, _ := runner.Run(ctx, "which "+native.Command)
	if code != 0 {
		return nil, false
	}
	return &Strategy{
		Kind:    "native",
		Command: native.Command,
		Args:    native.Args,
	}, true
}

// BuildCommand assembles the full shell command string from a Strategy.
// For Docker, wraps in "docker exec -it <containerID> <cmd> <args...>".
// Optional extra args are appended after the plugin-defined args.
// Args containing spaces are single-quoted for safe shell interpretation.
func (s *Strategy) BuildCommand(extra ...string) string {
	var parts []string
	if s.Kind == "docker" {
		parts = []string{"docker", "exec", "-it", s.ContainerID, s.Command}
	} else {
		parts = []string{s.Command}
	}
	for _, a := range append(s.Args, extra...) {
		if strings.ContainsAny(a, " \t;|&<>\"") {
			parts = append(parts, "'"+strings.ReplaceAll(a, "'", "'\\''")+"'")
		} else {
			parts = append(parts, a)
		}
	}
	return strings.Join(parts, " ")
}
