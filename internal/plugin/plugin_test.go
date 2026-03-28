package plugin_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emusal/alogin2/internal/plugin"
)

// ── mock implementations ──────────────────────────────────────────────────────

type mockVault struct{ data map[string]string }

func (m *mockVault) Get(account string) (string, error) {
	if v, ok := m.data[account]; ok {
		return v, nil
	}
	return "", fmt.Errorf("not found: %s", account)
}
func (m *mockVault) Set(account, password string) error { return nil }
func (m *mockVault) Delete(account string) error        { return nil }
func (m *mockVault) Name() string                       { return "mock" }

type mockRunner struct {
	runOutputs map[string]string // cmd prefix → output
	ptyOutput  string
}

func (r *mockRunner) Run(_ context.Context, cmd string) (string, int, error) {
	for prefix, out := range r.runOutputs {
		if len(cmd) >= len(prefix) && cmd[:len(prefix)] == prefix {
			return out, 0, nil
		}
	}
	return "", 1, nil // not found → exit 1
}

func (r *mockRunner) RunPTY(_ context.Context, cmd string, rules []plugin.PTYRule) (string, error) {
	return r.ptyOutput, nil
}

func (r *mockRunner) RunInteractive(_ context.Context, cmd string, rules []plugin.PTYRule, env map[string]string) error {
	return nil
}

// ── sample YAML ───────────────────────────────────────────────────────────────

const mariadbYAML = `
name: "mariadb"
version: "1"
auth:
  provider: "vault"
  mapping:
    - var: "DB_PASS"
      path: "root@dbhost:db_password"
      expose: "prompt"
      automation:
        expect: "Enter password:"
        send: "{{DB_PASS}}"
        send_newline: true
runtime:
  strategies: ["docker", "native"]
  environments:
    native:
      command: "mysql"
      args: ["-u", "root", "-p"]
    docker:
      container_match: "mariadb"
      command: "mysql"
      args: ["-u", "root", "-p"]
`

func writeYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestLoadFromFile_Valid(t *testing.T) {
	p, err := plugin.LoadFromFile(writeYAML(t, mariadbYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name != "mariadb" {
		t.Errorf("name = %q, want mariadb", p.Name)
	}
	if p.Auth.Provider != plugin.AuthProviderVault {
		t.Errorf("provider = %q, want vault", p.Auth.Provider)
	}
	if len(p.Auth.Mapping) != 1 {
		t.Fatalf("mapping len = %d, want 1", len(p.Auth.Mapping))
	}
	m := p.Auth.Mapping[0]
	if m.Var != "DB_PASS" {
		t.Errorf("var = %q, want DB_PASS", m.Var)
	}
	if m.Automation == nil {
		t.Fatal("automation is nil")
	}
	if m.Automation.Expect != "Enter password:" {
		t.Errorf("expect = %q", m.Automation.Expect)
	}
	if !m.Automation.SendNewline {
		t.Error("send_newline should be true")
	}
	if len(p.Runtime.Strategies) != 2 {
		t.Errorf("strategies len = %d, want 2", len(p.Runtime.Strategies))
	}
}

func TestLoadFromFile_MissingName(t *testing.T) {
	y := `
version: "1"
runtime:
  strategies: ["native"]
  environments:
    native:
      command: "mysql"
      args: []
`
	_, err := plugin.LoadFromFile(writeYAML(t, y))
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestLoadFromFile_MissingAutomation(t *testing.T) {
	y := `
name: "bad"
version: "1"
auth:
  provider: "vault"
  mapping:
    - var: "PASS"
      path: "user@host:pass"
      expose: "prompt"
runtime:
  strategies: ["native"]
  environments:
    native:
      command: "mysql"
      args: []
`
	_, err := plugin.LoadFromFile(writeYAML(t, y))
	if err == nil {
		t.Fatal("expected error: prompt without automation")
	}
}

func TestLoadDir(t *testing.T) {
	dir := t.TempDir()
	// Two valid plugins
	os.WriteFile(filepath.Join(dir, "mariadb.yaml"), []byte(mariadbYAML), 0600)
	os.WriteFile(filepath.Join(dir, "redis.yaml"), []byte(strings.Replace(mariadbYAML, "mariadb", "redis", 1)), 0600)
	// One invalid (no name)
	os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("version: 1\n"), 0600)

	plugins, err := plugin.LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir error: %v", err)
	}
	if len(plugins) != 2 {
		t.Errorf("loaded %d plugins, want 2", len(plugins))
	}
}

func TestResolveSecrets_Vault(t *testing.T) {
	p, _ := plugin.LoadFromFile(writeYAML(t, mariadbYAML))
	vlt := &mockVault{data: map[string]string{"root@dbhost:db_password": "s3cret"}}
	secrets, err := plugin.ResolveSecrets(p, vlt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secrets["DB_PASS"] != "s3cret" {
		t.Errorf("DB_PASS = %q, want s3cret", secrets["DB_PASS"])
	}
}

func TestResolveSecrets_Env(t *testing.T) {
	y := `
name: "envtest"
version: "1"
auth:
  provider: "env"
  mapping:
    - var: "MY_TOKEN"
      path: "MY_TEST_TOKEN_VAR"
      expose: "env"
runtime:
  strategies: ["native"]
  environments:
    native:
      command: "curl"
      args: []
`
	t.Setenv("MY_TEST_TOKEN_VAR", "tok123")
	p, _ := plugin.LoadFromFile(writeYAML(t, y))
	secrets, err := plugin.ResolveSecrets(p, &mockVault{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secrets["MY_TOKEN"] != "tok123" {
		t.Errorf("MY_TOKEN = %q, want tok123", secrets["MY_TOKEN"])
	}
}

func TestResolveSecrets_Static(t *testing.T) {
	y := `
name: "statictest"
version: "1"
auth:
  provider: "static"
  mapping:
    - var: "API_KEY"
      path: "hardcoded-value"
      expose: "arg"
runtime:
  strategies: ["native"]
  environments:
    native:
      command: "tool"
      args: []
`
	p, _ := plugin.LoadFromFile(writeYAML(t, y))
	secrets, err := plugin.ResolveSecrets(p, &mockVault{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secrets["API_KEY"] != "hardcoded-value" {
		t.Errorf("API_KEY = %q, want hardcoded-value", secrets["API_KEY"])
	}
}

func TestApplyTemplate(t *testing.T) {
	secrets := plugin.Secrets{"DB_PASS": "mypassword", "USER": "admin"}
	result := plugin.ApplyTemplate("pass={{DB_PASS}} user={{USER}} other={{UNKNOWN}}", secrets)
	want := "pass=mypassword user=admin other={{UNKNOWN}}"
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

func TestDetectStrategy_DockerFirst(t *testing.T) {
	p, _ := plugin.LoadFromFile(writeYAML(t, mariadbYAML))
	runner := &mockRunner{
		runOutputs: map[string]string{
			"docker ps": "abc123 mariadb:10.6 mariadb-prod\n",
		},
	}
	s, err := plugin.DetectStrategy(context.Background(), p, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Kind != "docker" {
		t.Errorf("kind = %q, want docker", s.Kind)
	}
	if s.ContainerID != "abc123" {
		t.Errorf("containerID = %q, want abc123", s.ContainerID)
	}
}

func TestDetectStrategy_DockerMiss_NativeHit(t *testing.T) {
	p, _ := plugin.LoadFromFile(writeYAML(t, mariadbYAML))
	runner := &mockRunner{
		runOutputs: map[string]string{
			"docker ps": "xyz789 nginx nginx\n", // no mariadb match
			"which":     "/usr/bin/mysql\n",
		},
	}
	s, err := plugin.DetectStrategy(context.Background(), p, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Kind != "native" {
		t.Errorf("kind = %q, want native", s.Kind)
	}
}

func TestDetectStrategy_NoneFound(t *testing.T) {
	p, _ := plugin.LoadFromFile(writeYAML(t, mariadbYAML))
	runner := &mockRunner{
		runOutputs: map[string]string{}, // everything fails
	}
	_, err := plugin.DetectStrategy(context.Background(), p, runner)
	if err == nil {
		t.Fatal("expected error when no strategy found")
	}
}

func TestBuildCommand_Docker(t *testing.T) {
	s := &plugin.Strategy{
		Kind:        "docker",
		ContainerID: "abc123",
		Command:     "mysql",
		Args:        []string{"-u", "root", "-p"},
	}
	got := s.BuildCommand()
	want := "docker exec -it abc123 mysql -u root -p"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildCommand_Native(t *testing.T) {
	s := &plugin.Strategy{
		Kind:    "native",
		Command: "mysql",
		Args:    []string{"-u", "root", "-p"},
	}
	got := s.BuildCommand()
	want := "mysql -u root -p"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildPTYRules(t *testing.T) {
	p, _ := plugin.LoadFromFile(writeYAML(t, mariadbYAML))
	vlt := &mockVault{data: map[string]string{"root@dbhost:db_password": "s3cret"}}
	secrets, _ := plugin.ResolveSecrets(p, vlt)
	sess := &plugin.Session{
		Plugin:   p,
		Strategy: &plugin.Strategy{Kind: "native", Command: "mysql"},
		Secrets:  secrets,
	}
	rules := sess.BuildPTYRules()
	if len(rules) != 1 {
		t.Fatalf("rules len = %d, want 1", len(rules))
	}
	if rules[0].Pattern != "Enter password:" {
		t.Errorf("pattern = %q", rules[0].Pattern)
	}
	if rules[0].Send != "s3cret" {
		t.Errorf("send = %q, want s3cret", rules[0].Send)
	}
	if !rules[0].EchoSuppressed {
		t.Error("EchoSuppressed should be true for prompt rules")
	}
}

func TestSession_EnvVars(t *testing.T) {
	y := `
name: "envtest"
version: "1"
auth:
  provider: "static"
  mapping:
    - var: "API_KEY"
      path: "mykey"
      expose: "env"
    - var: "API_SECRET"
      path: "mysecret"
      expose: "arg"
runtime:
  strategies: ["native"]
  environments:
    native:
      command: "tool"
      args: []
`
	p, _ := plugin.LoadFromFile(writeYAML(t, y))
	secrets, _ := plugin.ResolveSecrets(p, &mockVault{})
	sess := &plugin.Session{Plugin: p, Strategy: &plugin.Strategy{Kind: "native"}, Secrets: secrets}
	env := sess.EnvVars()
	if len(env) != 1 {
		t.Errorf("env len = %d, want 1", len(env))
	}
	if env["API_KEY"] != "mykey" {
		t.Errorf("API_KEY = %q, want mykey", env["API_KEY"])
	}
	if _, ok := env["API_SECRET"]; ok {
		t.Error("API_SECRET should not be in env vars (it's arg-expose)")
	}
}

func TestSession_AuditVars(t *testing.T) {
	p, _ := plugin.LoadFromFile(writeYAML(t, mariadbYAML))
	secrets := plugin.Secrets{"DB_PASS": "s3cret"}
	sess := &plugin.Session{Plugin: p, Strategy: &plugin.Strategy{}, Secrets: secrets}
	names := sess.AuditVars()
	if len(names) != 1 {
		t.Errorf("audit vars len = %d, want 1", len(names))
	}
	if names[0] != "DB_PASS" {
		t.Errorf("audit var = %q, want DB_PASS", names[0])
	}
}

func TestPrepare(t *testing.T) {
	p, _ := plugin.LoadFromFile(writeYAML(t, mariadbYAML))
	vlt := &mockVault{data: map[string]string{"root@dbhost:db_password": "s3cret"}}
	runner := &mockRunner{
		runOutputs: map[string]string{
			"docker ps": "abc123 mariadb:10.6 mariadb-prod\n",
		},
	}
	sess, err := plugin.Prepare(context.Background(), p, vlt, runner)
	if err != nil {
		t.Fatalf("Prepare error: %v", err)
	}
	if sess.Strategy.Kind != "docker" {
		t.Errorf("strategy kind = %q, want docker", sess.Strategy.Kind)
	}
	if sess.Secrets["DB_PASS"] != "s3cret" {
		t.Errorf("DB_PASS = %q, want s3cret", sess.Secrets["DB_PASS"])
	}
}
