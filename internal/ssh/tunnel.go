package ssh

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
)

// TunnelSpec describes one port-forward mapping.
type TunnelSpec struct {
	// For -L: LocalHost:LocalPort → RemoteHost:RemotePort
	// For -R: RemoteHost:RemotePort → LocalHost:LocalPort
	LocalHost  string
	LocalPort  int
	RemoteHost string
	RemotePort int
}

// ForwardLocal sets up a local→remote port forward (-L).
// Listens on localhost:LocalPort, forwards connections to RemoteHost:RemotePort
// through the SSH connection.
func (c *Client) ForwardLocal(ctx context.Context, spec TunnelSpec) error {
	localAddr := fmt.Sprintf("%s:%d", spec.LocalHost, spec.LocalPort)
	if spec.LocalHost == "" {
		localAddr = fmt.Sprintf("127.0.0.1:%d", spec.LocalPort)
	}
	ln, err := net.Listen("tcp", localAddr)
	if err != nil {
		return fmt.Errorf("local listener %s: %w", localAddr, err)
	}

	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	go func() {
		for {
			local, err := ln.Accept()
			if err != nil {
				return
			}
			remoteAddr := fmt.Sprintf("%s:%d", spec.RemoteHost, spec.RemotePort)
			go func(local net.Conn) {
				defer local.Close()
				remote, err := c.inner.Dial("tcp", remoteAddr)
				if err != nil {
					return
				}
				defer remote.Close()
				tunnel(local, remote)
			}(local)
		}
	}()

	return nil
}

// ForwardRemote sets up a remote→local port forward (-R).
// Asks the SSH server to listen on RemoteHost:RemotePort and forward
// incoming connections to LocalHost:LocalPort.
func (c *Client) ForwardRemote(ctx context.Context, spec TunnelSpec) error {
	remoteAddr := fmt.Sprintf("%s:%d", spec.RemoteHost, spec.RemotePort)
	ln, err := c.inner.Listen("tcp", remoteAddr)
	if err != nil {
		return fmt.Errorf("remote listener %s: %w", remoteAddr, err)
	}

	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	go func() {
		for {
			remote, err := ln.Accept()
			if err != nil {
				return
			}
			localAddr := fmt.Sprintf("%s:%d", spec.LocalHost, spec.LocalPort)
			go func(remote net.Conn) {
				defer remote.Close()
				local, err := net.Dial("tcp", localAddr)
				if err != nil {
					return
				}
				defer local.Close()
				tunnel(remote, local)
			}(remote)
		}
	}()

	return nil
}

func tunnel(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); io.Copy(a, b) }()
	go func() { defer wg.Done(); io.Copy(b, a) }()
	wg.Wait()
}
