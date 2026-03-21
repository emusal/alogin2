package ssh

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
)

// SFTPClient wraps pkg/sftp for file transfers.
type SFTPClient struct {
	cl *sftp.Client
}

func newSFTPClient(c *Client) (*SFTPClient, error) {
	sc, err := sftp.NewClient(c.inner)
	if err != nil {
		return nil, fmt.Errorf("sftp client: %w", err)
	}
	return &SFTPClient{cl: sc}, nil
}

// Close closes the SFTP connection.
func (s *SFTPClient) Close() error { return s.cl.Close() }

// InteractiveSFTP launches the system sftp binary for an interactive session,
// building -J (ProxyJump) and -P (port) flags from the hop chain.
// Authentication is handled by the system sftp binary (SSH agent / keys / prompts).
func InteractiveSFTP(hops []HopConfig) error {
	dest := hops[len(hops)-1]

	var args []string

	// ProxyJump for multi-hop
	if len(hops) > 1 {
		jumps := make([]string, len(hops)-1)
		for i, h := range hops[:len(hops)-1] {
			jumps[i] = fmt.Sprintf("%s@%s:%d", h.User, h.Host, h.Port)
		}
		args = append(args, "-J", strings.Join(jumps, ","))
	}

	args = append(args, "-P", fmt.Sprint(dest.Port))
	args = append(args, fmt.Sprintf("%s@%s", dest.User, dest.Host))

	cmd := exec.Command("sftp", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Upload copies a local file to the remote path.
func (s *SFTPClient) Upload(localPath, remotePath string) error {
	src, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local %s: %w", localPath, err)
	}
	defer src.Close()

	// Ensure remote directory exists
	if err := s.cl.MkdirAll(filepath.Dir(remotePath)); err != nil {
		return fmt.Errorf("mkdir remote %s: %w", filepath.Dir(remotePath), err)
	}

	dst, err := s.cl.Create(remotePath)
	if err != nil {
		return fmt.Errorf("create remote %s: %w", remotePath, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("upload %s: %w", localPath, err)
	}
	fmt.Printf("Uploaded: %s → %s\n", localPath, remotePath)
	return nil
}

// Download copies a remote file to a local path.
func (s *SFTPClient) Download(remotePath, localPath string) error {
	src, err := s.cl.Open(remotePath)
	if err != nil {
		return fmt.Errorf("open remote %s: %w", remotePath, err)
	}
	defer src.Close()

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("mkdir local %s: %w", filepath.Dir(localPath), err)
	}

	dst, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("create local %s: %w", localPath, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("download %s: %w", remotePath, err)
	}
	fmt.Printf("Downloaded: %s → %s\n", remotePath, localPath)
	return nil
}
