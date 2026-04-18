package sshconfig

import (
	"strings"
	"testing"
)

func TestParse_BasicBlock(t *testing.T) {
	input := `
Host web-01
    HostName 10.0.0.1
    User deploy
    Port 2222
    IdentityFile ~/.ssh/id_ed25519
`
	cfg, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Blocks) != 1 {
		t.Fatalf("want 1 block, got %d", len(cfg.Blocks))
	}
	b := cfg.Blocks[0]
	if b.Pattern != "web-01" {
		t.Errorf("Pattern: want web-01, got %q", b.Pattern)
	}
	if b.HostName != "10.0.0.1" {
		t.Errorf("HostName: want 10.0.0.1, got %q", b.HostName)
	}
	if b.User != "deploy" {
		t.Errorf("User: want deploy, got %q", b.User)
	}
	if b.Port != 2222 {
		t.Errorf("Port: want 2222, got %d", b.Port)
	}
	if b.IdentityFile == "" || !strings.HasSuffix(b.IdentityFile, "/id_ed25519") {
		t.Errorf("IdentityFile unexpected: %q", b.IdentityFile)
	}
	if b.IsWildcard {
		t.Error("IsWildcard should be false for plain name")
	}
}

func TestParse_WildcardBlock(t *testing.T) {
	input := `
Host *
    ServerAliveInterval 60

Host *.example.com
    User admin
`
	cfg, _ := Parse(strings.NewReader(input))
	for _, b := range cfg.Blocks {
		if !b.IsWildcard {
			t.Errorf("block %q should be wildcard", b.Pattern)
		}
	}
}

func TestParse_HostnameFallback(t *testing.T) {
	input := `
Host bastion
    User ops
`
	cfg, _ := Parse(strings.NewReader(input))
	if cfg.Blocks[0].HostName != "bastion" {
		t.Errorf("HostName should default to Pattern, got %q", cfg.Blocks[0].HostName)
	}
}

func TestParse_ProxyJump(t *testing.T) {
	input := `
Host internal
    HostName 192.168.1.10
    ProxyJump bastion.example.com
`
	cfg, _ := Parse(strings.NewReader(input))
	b := cfg.Blocks[0]
	if b.ProxyJump != "bastion.example.com" {
		t.Errorf("ProxyJump: want bastion.example.com, got %q", b.ProxyJump)
	}
}

func TestParse_ProxyCommand(t *testing.T) {
	input := `
Host legacy
    HostName 192.168.1.20
    ProxyCommand ssh -W %h:%p jump.example.com
`
	cfg, _ := Parse(strings.NewReader(input))
	b := cfg.Blocks[0]
	if b.ProxyCommand != "ssh -W %h:%p jump.example.com" {
		t.Errorf("ProxyCommand: got %q", b.ProxyCommand)
	}
}

func TestParse_EqualsSeparator(t *testing.T) {
	input := `
Host db
    HostName=db.example.com
    User=root
`
	cfg, _ := Parse(strings.NewReader(input))
	b := cfg.Blocks[0]
	if b.HostName != "db.example.com" {
		t.Errorf("HostName: want db.example.com, got %q", b.HostName)
	}
	if b.User != "root" {
		t.Errorf("User: want root, got %q", b.User)
	}
}

func TestParse_Comments(t *testing.T) {
	input := `
# global comment
Host proxy
    # inline comment
    HostName proxy.example.com
`
	cfg, _ := Parse(strings.NewReader(input))
	if len(cfg.Blocks) != 1 {
		t.Fatalf("want 1 block, got %d", len(cfg.Blocks))
	}
}

// --- ParseProxyJump ---

func TestParseProxyJump_Single(t *testing.T) {
	hops := ParseProxyJump("bastion.example.com")
	if len(hops) != 1 || hops[0].Host != "bastion.example.com" {
		t.Errorf("unexpected hops: %+v", hops)
	}
}

func TestParseProxyJump_UserPort(t *testing.T) {
	hops := ParseProxyJump("ops@bastion.example.com:2222")
	if len(hops) != 1 {
		t.Fatalf("want 1 hop, got %d", len(hops))
	}
	h := hops[0]
	if h.Host != "bastion.example.com" || h.User != "ops" || h.Port != 2222 {
		t.Errorf("unexpected hop: %+v", h)
	}
}

func TestParseProxyJump_MultiHop(t *testing.T) {
	hops := ParseProxyJump("b1.example.com,b2.example.com")
	if len(hops) != 2 {
		t.Fatalf("want 2 hops, got %d", len(hops))
	}
	if hops[0].Host != "b1.example.com" || hops[1].Host != "b2.example.com" {
		t.Errorf("unexpected hops: %+v", hops)
	}
}

func TestParseProxyJump_None(t *testing.T) {
	if hops := ParseProxyJump("none"); hops != nil {
		t.Errorf("none should return nil, got %+v", hops)
	}
	if hops := ParseProxyJump(""); hops != nil {
		t.Errorf("empty should return nil")
	}
}

// --- ParseProxyCommand ---

func TestParseProxyCommand_WFlag(t *testing.T) {
	hop, ok := ParseProxyCommand("ssh -W %h:%p bastion.example.com")
	if !ok || hop.Host != "bastion.example.com" {
		t.Errorf("unexpected: ok=%v hop=%+v", ok, hop)
	}
}

func TestParseProxyCommand_HostFirst(t *testing.T) {
	hop, ok := ParseProxyCommand("ssh bastion.example.com -W %h:%p")
	if !ok || hop.Host != "bastion.example.com" {
		t.Errorf("unexpected: ok=%v hop=%+v", ok, hop)
	}
}

func TestParseProxyCommand_WithPort(t *testing.T) {
	hop, ok := ParseProxyCommand("ssh -p 2222 bastion.example.com -W %h:%p")
	if !ok || hop.Host != "bastion.example.com" || hop.Port != 2222 {
		t.Errorf("unexpected: ok=%v hop=%+v", ok, hop)
	}
}

func TestParseProxyCommand_NonSSH(t *testing.T) {
	_, ok := ParseProxyCommand("nc %h %p")
	if ok {
		t.Error("nc command should not parse as jump hop")
	}
}

func TestParseProxyCommand_Empty(t *testing.T) {
	_, ok := ParseProxyCommand("")
	if ok {
		t.Error("empty should return false")
	}
}

// --- containsGlob ---

func TestContainsGlob(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"*", true},
		{"*.example.com", true},
		{"host?", true},
		{"[abc]", true},
		{"web-01", false},
		{"bastion", false},
		{"192.168.1.1", false},
		{"two words", true}, // space → treated as wildcard/ambiguous
	}
	for _, c := range cases {
		if got := containsGlob(c.s); got != c.want {
			t.Errorf("containsGlob(%q) = %v, want %v", c.s, got, c.want)
		}
	}
}
