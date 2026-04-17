package ssh

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
)

// UploadDir recursively uploads localDir to remotePath.
// remotePath is created if it does not exist.
func (s *SFTPClient) UploadDir(localDir, remotePath string) error {
	remotePath = s.expandRemoteTilde(remotePath)
	return filepath.Walk(localDir, func(localFile string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(localDir, localFile)
		if err != nil {
			return err
		}
		remote := path.Join(remotePath, filepath.ToSlash(rel))
		if info.IsDir() {
			return s.cl.MkdirAll(remote)
		}
		return s.uploadFile(localFile, remote)
	})
}

// DownloadDir recursively downloads remotePath into localDir.
// localDir is created if it does not exist.
func (s *SFTPClient) DownloadDir(remotePath, localDir string) error {
	remotePath = s.expandRemoteTilde(remotePath)
	walker := s.cl.Walk(remotePath)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return fmt.Errorf("walk remote %s: %w", remotePath, err)
		}
		rel, err := filepath.Rel(remotePath, walker.Path())
		if err != nil {
			return err
		}
		local := filepath.Join(localDir, rel)
		if walker.Stat().IsDir() {
			if err := os.MkdirAll(local, 0755); err != nil {
				return fmt.Errorf("mkdir local %s: %w", local, err)
			}
			continue
		}
		if err := s.downloadFile(walker.Path(), local); err != nil {
			return err
		}
	}
	return nil
}

// uploadFile is the single-file core shared by Upload and UploadDir.
func (s *SFTPClient) uploadFile(localPath, remotePath string) error {
	if err := s.cl.MkdirAll(path.Dir(remotePath)); err != nil {
		return fmt.Errorf("mkdir remote %s: %w", path.Dir(remotePath), err)
	}
	src, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local %s: %w", localPath, err)
	}
	defer src.Close()
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

// downloadFile is the single-file core shared by Download and DownloadDir.
func (s *SFTPClient) downloadFile(remotePath, localPath string) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("mkdir local %s: %w", filepath.Dir(localPath), err)
	}
	src, err := s.cl.Open(remotePath)
	if err != nil {
		return fmt.Errorf("open remote %s: %w", remotePath, err)
	}
	defer src.Close()
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

// expandRemoteTilde resolves a leading ~ or ~/ in a remote path to the SFTP
// server's working directory (which is the home directory for most servers).
func (s *SFTPClient) expandRemoteTilde(p string) string {
	if p == "~" {
		if home, err := s.cl.Getwd(); err == nil {
			return home
		}
		return p
	}
	if strings.HasPrefix(p, "~/") {
		if home, err := s.cl.Getwd(); err == nil {
			return home + "/" + p[2:]
		}
	}
	return p
}

// UploadContent writes p directly to remotePath without needing a local file.
func (s *SFTPClient) UploadContent(p []byte, remotePath string) error {
	remotePath = s.expandRemoteTilde(remotePath)
	if err := s.cl.MkdirAll(path.Dir(remotePath)); err != nil {
		return fmt.Errorf("mkdir remote %s: %w", path.Dir(remotePath), err)
	}
	dst, err := s.cl.Create(remotePath)
	if err != nil {
		return fmt.Errorf("create remote %s: %w", remotePath, err)
	}
	defer dst.Close()
	if _, err := dst.Write(p); err != nil {
		return fmt.Errorf("write %s: %w", remotePath, err)
	}
	// Make executable so the caller can run it directly.
	return s.cl.Chmod(remotePath, 0700)
}

// Upload copies a local file to the remote path.
// If remotePath is an existing directory (or ends with / or .), the local
// filename is appended automatically, mirroring scp(1) behaviour.
func (s *SFTPClient) Upload(localPath, remotePath string) error {
	remotePath = s.expandRemoteTilde(remotePath)
	if info, err := s.cl.Stat(remotePath); err == nil && info.IsDir() {
		remotePath = path.Join(remotePath, filepath.Base(localPath))
	} else if strings.HasSuffix(remotePath, "/") || strings.HasSuffix(remotePath, "/.") || remotePath == "." {
		remotePath = path.Join(remotePath, filepath.Base(localPath))
	}
	return s.uploadFile(localPath, remotePath)
}

// Download copies a remote file to a local path.
func (s *SFTPClient) Download(remotePath, localPath string) error {
	remotePath = s.expandRemoteTilde(remotePath)
	return s.downloadFile(remotePath, localPath)
}
