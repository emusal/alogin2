//go:build integration

// Integration tests for the plugin package.
//
// These tests require the testenv Docker Compose environment to be running:
//
//	cd testenv && docker-compose up -d --build
//
// Run with:
//
//	go test -tags integration -v ./internal/plugin/...
//
// Environment variables (optional overrides):
//
//	ALOGIN_TEST_HOST     SSH host for target-mariadb (default: localhost)
//	ALOGIN_TEST_PORT     SSH port (default: 2222 via bastion; direct tests use port 22 on back_net)
//	ALOGIN_TEST_USER     SSH user (default: testuser)
//	ALOGIN_TEST_PASS     SSH password (default: testuser)
//	ALOGIN_TEST_DB_PASS  MariaDB root password (default: testpass)
//	ALOGIN_TEST_REDIS_PASS Redis password (default: testpass)

package plugin_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/emusal/alogin2/internal/plugin"
	gossh "golang.org/x/crypto/ssh"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// liveRunner connects to a real SSH host and implements plugin.RemoteRunner.
// Uses golang.org/x/crypto/ssh directly to avoid importing internal/ssh
// (which would create a test dependency cycle).
type liveRunner struct {
	host string
	port string
	user string
	pass string
}

func newLiveRunner(t *testing.T) *liveRunner {
	t.Helper()
	return &liveRunner{
		host: envOr("ALOGIN_TEST_HOST", "localhost"),
		port: envOr("ALOGIN_TEST_PORT", "2222"),
		user: envOr("ALOGIN_TEST_USER", "testuser"),
		pass: envOr("ALOGIN_TEST_PASS", "testuser"),
	}
}

func (r *liveRunner) Run(ctx context.Context, cmd string) (string, int, error) {
	return sshRun(ctx, r.host, r.port, r.user, r.pass, cmd)
}

func (r *liveRunner) RunPTY(ctx context.Context, cmd string, rules []plugin.PTYRule) (string, error) {
	return sshRunPTY(ctx, r.host, r.port, r.user, r.pass, cmd, rules)
}

// ── SSH helpers (uses golang.org/x/crypto/ssh directly) ─────────────────────

func sshDial(host, port, user, pass string) (*gossh.Client, error) {
	cfg := &gossh.ClientConfig{
		User:            user,
		Auth:            []gossh.AuthMethod{gossh.Password(pass)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}
	return gossh.Dial("tcp", net.JoinHostPort(host, port), cfg)
}

func sshRun(ctx context.Context, host, port, user, pass, cmd string) (string, int, error) {
	cl, err := sshDial(host, port, user, pass)
	if err != nil {
		return "", -1, fmt.Errorf("ssh dial: %w", err)
	}
	defer cl.Close()

	sess, err := cl.NewSession()
	if err != nil {
		return "", -1, err
	}
	defer sess.Close()

	out, err := sess.CombinedOutput(cmd)
	if err != nil {
		if exitErr, ok := err.(*gossh.ExitError); ok {
			return string(out), exitErr.ExitStatus(), nil
		}
		return string(out), -1, err
	}
	return string(out), 0, nil
}

func sshRunPTY(ctx context.Context, host, port, user, pass, cmd string, rules []plugin.PTYRule) (string, error) {
	cl, err := sshDial(host, port, user, pass)
	if err != nil {
		return "", fmt.Errorf("ssh dial: %w", err)
	}
	defer cl.Close()

	sess, err := cl.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()

	if err := sess.RequestPty("xterm", 24, 80, gossh.TerminalModes{
		gossh.ECHO: 1, gossh.TTY_OP_ISPEED: 14400, gossh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		return "", err
	}

	stdinPipe, err := sess.StdinPipe()
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	ew := &integrationExpectWriter{
		buf:   &buf,
		rules: rules,
		used:  make([]bool, len(rules)),
		stdin: stdinPipe,
	}
	sess.Stdout = ew
	sess.Stderr = ew

	if err := sess.Start(cmd); err != nil {
		return "", err
	}
	_ = sess.Wait()
	return buf.String(), nil
}

type integrationExpectWriter struct {
	buf   *strings.Builder
	rules []plugin.PTYRule
	used  []bool
	stdin interface{ Write([]byte) (int, error) }
}

func (w *integrationExpectWriter) Write(p []byte) (int, error) {
	n, err := w.buf.Write(p)
	recent := w.buf.String()
	for i, rule := range w.rules {
		if w.used[i] {
			continue
		}
		if strings.Contains(recent, rule.Pattern) {
			send := rule.Send
			if rule.SendNewline {
				send += "\n"
			}
			_, _ = w.stdin.Write([]byte(send))
			w.used[i] = true
		}
	}
	return n, err
}

// ── tests ─────────────────────────────────────────────────────────────────────

// TestIntegration_LoadPluginDir verifies that sample plugin YAMLs in testenv/plugins/
// all parse and validate without errors.
func TestIntegration_LoadPluginDir(t *testing.T) {
	dir := "../../testenv/plugins"
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Skip("testenv/plugins not found")
	}
	plugins, err := plugin.LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir error: %v", err)
	}
	if len(plugins) == 0 {
		t.Error("no plugins loaded from testenv/plugins/")
	}
	for _, p := range plugins {
		t.Logf("loaded plugin: %s (v%s, strategies: %v)", p.Name, p.Version, p.Runtime.Strategies)
	}
}

// TestIntegration_DetectStrategy_MariaDB connects to target-mariadb (via bastion)
// and verifies that the native `mysql` binary is detected.
func TestIntegration_DetectStrategy_MariaDB(t *testing.T) {
	p, err := plugin.LoadFromFile("../../testenv/plugins/mariadb.yaml")
	if err != nil {
		t.Fatalf("load plugin: %v", err)
	}

	runner := newLiveRunner(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check that the mariadb target is reachable.
	if _, _, err := runner.Run(ctx, "echo ok"); err != nil {
		t.Skipf("target not reachable: %v", err)
	}

	s, err := plugin.DetectStrategy(ctx, p, runner)
	if err != nil {
		t.Fatalf("DetectStrategy: %v", err)
	}
	t.Logf("strategy: kind=%s containerID=%s cmd=%s", s.Kind, s.ContainerID, s.Command)
	if s.Command != "mysql" {
		t.Errorf("expected command mysql, got %q", s.Command)
	}
}

// TestIntegration_ResolveSecrets_Static verifies that secrets are resolved
// correctly using the static provider (no vault required).
func TestIntegration_ResolveSecrets_Static(t *testing.T) {
	y := `
name: "statictest"
version: "1"
auth:
  provider: "static"
  mapping:
    - var: "DB_PASS"
      path: "hardcoded"
      expose: "arg"
runtime:
  strategies: ["native"]
  environments:
    native:
      command: "echo"
      args: ["{{DB_PASS}}"]
`
	path := writeYAML(t, y)
	p, err := plugin.LoadFromFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	secrets, err := plugin.ResolveSecrets(p, &mockVault{})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if secrets["DB_PASS"] != "hardcoded" {
		t.Errorf("got %q, want hardcoded", secrets["DB_PASS"])
	}
}

// TestIntegration_Prepare_MariaDB runs the full Prepare() flow on target-mariadb.
// Uses static provider so no vault setup is required in testenv.
func TestIntegration_Prepare_MariaDB(t *testing.T) {
	dbPass := envOr("ALOGIN_TEST_DB_PASS", "testpass")

	y := fmt.Sprintf(`
name: "mariadb-static"
version: "1"
auth:
  provider: "static"
  mapping:
    - var: "DB_PASS"
      path: "%s"
      expose: "prompt"
      automation:
        expect: "Enter password:"
        send: "{{DB_PASS}}"
        send_newline: true
runtime:
  strategies: ["native"]
  environments:
    native:
      command: "mysql"
      args: ["-u", "root", "-p"]
`, dbPass)

	path := writeYAML(t, y)
	p, err := plugin.LoadFromFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	runner := newLiveRunner(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, _, err := runner.Run(ctx, "which mysql"); err != nil {
		t.Skipf("mysql not available on target: %v", err)
	}

	sess, err := plugin.Prepare(ctx, p, &mockVault{}, runner)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if sess.Strategy.Kind != "native" {
		t.Errorf("kind = %q, want native", sess.Strategy.Kind)
	}
	t.Logf("prepared session: strategy=%s cmd=%s", sess.Strategy.Kind, sess.Strategy.BuildCommand())
	t.Logf("audit vars: %v", sess.AuditVars())
}
