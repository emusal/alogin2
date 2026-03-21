// Package ssh provides a native Go SSH client that replaces conn.exp.
// It supports multi-hop ProxyJump, interactive PTY sessions, port tunneling,
// SFTP file transfer, SSHFS mounts, and Docker/Vagrant connectivity.
package ssh

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// ErrDialViaEOF is returned by DialVia when the inner hop closes the connection
// during handshake, typically due to server-side TCP forwarding restrictions.
// Callers can detect this with errors.As to trigger a shell-chain fallback.
type ErrDialViaEOF struct {
	ProxyAddr string
	DestAddr  string
}

func (e *ErrDialViaEOF) Error() string {
	return fmt.Sprintf(
		"connection to %s (via %s) closed before SSH handshake completed\n"+
			"  Possible causes:\n"+
			"  - AllowTcpForwarding disabled on %s\n"+
			"  - PermitOpen on %s does not allow forwarding to %s\n"+
			"  - %s is unreachable from %s\n"+
			"  Verify: ssh -J %s user@%s",
		e.DestAddr, e.ProxyAddr,
		e.ProxyAddr,
		e.ProxyAddr, e.DestAddr,
		e.DestAddr, e.ProxyAddr,
		e.ProxyAddr, e.DestAddr,
	)
}

// HopConfig holds everything needed to authenticate at one SSH hop.
type HopConfig struct {
	Host     string
	Port     int
	User     string
	Password string // empty = try key auth only
	KeyPath  string // path to private key; empty = use ssh-agent
	Timeout  time.Duration
}

// Addr returns "host:port".
func (h HopConfig) Addr() string {
	return fmt.Sprintf("%s:%d", h.Host, h.Port)
}

// Client wraps an active SSH connection to the final destination.
type Client struct {
	inner *gossh.Client
}

// Close tears down the connection.
func (c *Client) Close() error {
	return c.inner.Close()
}

// Dial connects directly (single hop) to a host.
func Dial(cfg HopConfig) (*Client, error) {
	sshCfg, err := makeSSHConfig(cfg)
	if err != nil {
		return nil, err
	}
	cl, err := gossh.Dial("tcp", cfg.Addr(), sshCfg)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", cfg.Addr(), err)
	}
	return &Client{inner: cl}, nil
}

// DialVia connects to dest through an already-established client (one ProxyJump hop).
//
// Host key checking is intentionally skipped for inner hops: the destination
// hostname (e.g. "localhost", "192.168.x.x") is relative to the proxy's
// network, not the local machine's, so local known_hosts entries are
// meaningless and would cause false "key mismatch" errors.
// The outer first hop (Dial) still enforces known_hosts verification.
func DialVia(proxy *Client, dest HopConfig) (*Client, error) {
	conn, err := proxy.inner.Dial("tcp", dest.Addr())
	if err != nil {
		return nil, fmt.Errorf("dial %s via proxy: %w", dest.Addr(), err)
	}

	sshCfg, err := makeSSHConfig(dest)
	if err != nil {
		conn.Close()
		return nil, err
	}
	// Inner hop: bypass local known_hosts (hostname is in the proxy's network context).
	sshCfg.HostKeyCallback = gossh.InsecureIgnoreHostKey()

	ncc, chans, reqs, err := gossh.NewClientConn(conn, dest.Addr(), sshCfg)
	if err != nil {
		conn.Close()
		if errors.Is(err, io.EOF) {
			return nil, &ErrDialViaEOF{
				ProxyAddr: proxy.inner.RemoteAddr().String(),
				DestAddr:  dest.Addr(),
			}
		}
		return nil, fmt.Errorf("ssh handshake %s: %w", dest.Addr(), err)
	}
	return &Client{inner: gossh.NewClient(ncc, chans, reqs)}, nil
}

// makeSSHConfig builds a gossh.ClientConfig for a hop.
func makeSSHConfig(cfg HopConfig) (*gossh.ClientConfig, error) {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	var authMethods []gossh.AuthMethod

	// 1. Try SSH agent
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if conn, err := net.Dial("unix", sock); err == nil {
			authMethods = append(authMethods, gossh.PublicKeysCallback(
				agent.NewClient(conn).Signers))
		}
	}

	// 2. Try explicit key file
	if cfg.KeyPath != "" {
		if am, err := publicKeyAuth(cfg.KeyPath); err == nil {
			authMethods = append(authMethods, am)
		}
	} else {
		// Try default key locations
		for _, kp := range defaultKeyPaths() {
			if am, err := publicKeyAuth(kp); err == nil {
				authMethods = append(authMethods, am)
				break
			}
		}
	}

	// 3. Password auth
	if cfg.Password != "" {
		authMethods = append(authMethods,
			gossh.Password(cfg.Password),
			gossh.KeyboardInteractive(passwordKeyboardInteractive(cfg.Password)),
		)
	}

	hostKeyCallback, err := hostKeyCallback()
	if err != nil {
		// Fall back to InsecureIgnoreHostKey in dev/migration scenarios
		hostKeyCallback = gossh.InsecureIgnoreHostKey()
	}

	return &gossh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         timeout,
	}, nil
}

func publicKeyAuth(keyPath string) (gossh.AuthMethod, error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	signer, err := gossh.ParsePrivateKey(data)
	if err != nil {
		return nil, err
	}
	return gossh.PublicKeys(signer), nil
}

func defaultKeyPaths() []string {
	home, _ := os.UserHomeDir()
	return []string{
		home + "/.ssh/id_ed25519",
		home + "/.ssh/id_rsa",
		home + "/.ssh/id_ecdsa",
	}
}

func hostKeyCallback() (gossh.HostKeyCallback, error) {
	home, _ := os.UserHomeDir()
	khPath := home + "/.ssh/known_hosts"

	// Ensure known_hosts file exists so knownhosts.New doesn't fail.
	if _, err := os.Stat(khPath); errors.Is(err, os.ErrNotExist) {
		if f, err := os.OpenFile(khPath, os.O_CREATE|os.O_WRONLY, 0600); err == nil {
			f.Close()
		}
	}

	checker, err := knownhosts.New(khPath)
	if err != nil {
		return gossh.InsecureIgnoreHostKey(), nil
	}

	return func(hostname string, remote net.Addr, key gossh.PublicKey) error {
		err := checker(hostname, remote, key)
		if err == nil {
			return nil
		}

		// Key mismatch (possible MITM) — always reject.
		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) && len(keyErr.Want) > 0 {
			return fmt.Errorf(
				"host key mismatch for %s\n"+
					"  Expected: %s\n"+
					"  If the server was reinstalled, run: ssh-keygen -R %s\n"+
					"  If this is a proxied/internal address, this is a false alarm — report it.",
				hostname, keyErr.Want[0].Key.Type(), hostname)
		}

		// Unknown host: auto-accept and add to known_hosts (mirrors ssh StrictHostKeyChecking=no).
		fp := gossh.FingerprintSHA256(key)
		fmt.Fprintf(os.Stderr, "Warning: Permanently added '%s' (%s) to the list of known hosts.\n", hostname, key.Type())
		_ = fp

		// Append the key to known_hosts.
		line := knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key)
		f, err := os.OpenFile(khPath, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not update known_hosts: %v\n", err)
			return nil
		}
		defer f.Close()
		fmt.Fprintln(f, line)
		fmt.Fprintf(os.Stderr, "Warning: Permanently added '%s' (%s) to the list of known hosts.\n", hostname, key.Type())
		return nil
	}, nil
}

func passwordKeyboardInteractive(password string) gossh.KeyboardInteractiveChallenge {
	return func(user, instruction string, questions []string, echos []bool) ([]string, error) {
		answers := make([]string, len(questions))
		for i := range questions {
			answers[i] = password
		}
		return answers, nil
	}
}

// Inner returns the underlying gossh.Client for advanced use.
func (c *Client) Inner() *gossh.Client { return c.inner }

// Run executes a command non-interactively and returns combined output.
func (c *Client) Run(command string) ([]byte, error) {
	sess, err := c.inner.NewSession()
	if err != nil {
		return nil, err
	}
	defer sess.Close()
	return sess.CombinedOutput(command)
}

// NewSession opens a raw SSH session on this connection.
func (c *Client) NewSession() (*gossh.Session, error) {
	return c.inner.NewSession()
}

// Dial opens a TCP connection through this SSH connection.
func (c *Client) Dial(network, addr string) (net.Conn, error) {
	return c.inner.Dial(network, addr)
}

// Listen opens a remote listener through this SSH connection.
func (c *Client) Listen(network, addr string) (net.Listener, error) {
	return c.inner.Listen(network, addr)
}

// SFTPClient returns a new SFTP client on this connection.
func (c *Client) SFTPClient() (*SFTPClient, error) {
	return newSFTPClient(c)
}

// ensure io.Closer satisfied
var _ io.Closer = (*Client)(nil)
