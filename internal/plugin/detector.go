package plugin

import (
	"context"
	"fmt"
	"strings"
)

// Strategy is a resolved execution strategy for a plugin session.
type Strategy struct {
	Kind        string   // "docker" | "docker-compose" | "native"
	DockerCmd   string   // docker/docker-compose: resolved docker binary path (absolute or "docker")
	ContainerID string   // docker: matched container ID
	ServiceName string   // docker-compose: matched service name
	ComposeFile string   // docker-compose: full path to compose YAML on remote host
	Command     string   // executable name
	Args        []string // command arguments
}

// commonDockerPaths lists well-known Docker binary locations probed when
// docker is not found via PATH in non-interactive SSH sessions.
var commonDockerPaths = []string{
	"/usr/local/bin/docker",
	"/usr/bin/docker",
	"/usr/local/sbin/docker",
}

// resolveDockerBinary returns the docker binary path usable on the remote host.
// It first tries "docker" via PATH (fast path for properly configured hosts),
// then falls back to probing common absolute installation paths for SSH sessions
// that do not load the user's profile (and therefore have a restricted PATH).
func resolveDockerBinary(ctx context.Context, runner RemoteRunner) string {
	if _, code, _ := runner.Run(ctx, "docker --version"); code == 0 {
		return "docker"
	}
	for _, p := range commonDockerPaths {
		if _, code, _ := runner.Run(ctx, "test -x "+p); code == 0 {
			return p
		}
	}
	return ""
}

// DetectStrategy probes the remote host and returns the first viable Strategy
// according to the plugin's strategy priority list.
func DetectStrategy(ctx context.Context, p *Plugin, runner RemoteRunner) (*Strategy, error) {
	// Resolve the docker binary path once for all docker-based strategies.
	// Non-interactive SSH sessions often have a restricted PATH that omits
	// Docker's install location, so we probe known absolute paths as a fallback.
	var dockerCmd string
	for _, kind := range p.Runtime.Strategies {
		if kind == "docker" || kind == "docker-compose" {
			dockerCmd = resolveDockerBinary(ctx, runner)
			break
		}
	}

	for _, kind := range p.Runtime.Strategies {
		switch kind {
		case "docker":
			if s, ok := detectDocker(ctx, p, runner, dockerCmd); ok {
				return s, nil
			}
		case "docker-compose":
			if s, ok := detectDockerCompose(ctx, p, runner, dockerCmd); ok {
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

func detectDocker(ctx context.Context, p *Plugin, runner RemoteRunner, dockerCmd string) (*Strategy, bool) {
	if dockerCmd == "" {
		return nil, false
	}
	docker := p.Runtime.Environments.Docker
	if docker == nil {
		return nil, false
	}
	out, code, err := runner.Run(ctx, dockerCmd+" ps --format '{{.ID}} {{.Image}} {{.Names}}'")
	if err != nil || code != 0 {
		return nil, false
	}
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
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
				DockerCmd:   dockerCmd,
				ContainerID: containerID,
				Command:     docker.Command,
				Args:        docker.Args,
			}, true
		}
	}
	return nil, false
}

func detectDockerCompose(ctx context.Context, p *Plugin, runner RemoteRunner, dockerCmd string) (*Strategy, bool) {
	if dockerCmd == "" {
		return nil, false
	}
	dc := p.Runtime.Environments.DockerCompose
	if dc == nil {
		return nil, false
	}
	fullPath := resolveComposeFile(dc.ComposeDir, dc.ComposeFile)
	out, code, err := runner.Run(ctx, dockerCmd+" compose -f "+fullPath+" ps --format '{{.Service}} {{.Status}}'")
	if err != nil || code != 0 {
		return nil, false
	}
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		if strings.Contains(strings.ToLower(line), strings.ToLower(dc.ServiceMatch)) {
			fields := strings.Fields(line)
			if len(fields) == 0 {
				continue
			}
			return &Strategy{
				Kind:        "docker-compose",
				DockerCmd:   dockerCmd,
				ServiceName: fields[0],
				ComposeFile: fullPath,
				Command:     dc.Command,
				Args:        dc.Args,
			}, true
		}
	}
	return nil, false
}

// resolveComposeFile builds the full compose file path from dir and file components.
// If composeFile is empty, docker-compose.yml is used as the default.
// If composeDir is empty, composeFile is used as-is (allows absolute paths).
func resolveComposeFile(composeDir, composeFile string) string {
	if composeFile == "" {
		composeFile = "docker-compose.yml"
	}
	if composeDir == "" {
		return composeFile
	}
	return composeDir + "/" + composeFile
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
	dc := s.DockerCmd
	if dc == "" {
		dc = "docker" // fallback for manually constructed Strategy values
	}
	var parts []string
	switch s.Kind {
	case "docker":
		parts = []string{dc, "exec", "-it", s.ContainerID, s.Command}
	case "docker-compose":
		parts = []string{dc, "compose", "-f", s.ComposeFile, "exec", s.ServiceName, s.Command}
	default:
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
