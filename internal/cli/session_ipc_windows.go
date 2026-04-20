//go:build windows

package cli

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// sessionListen binds to a random TCP loopback port and writes the address
// to the session sock file so the exec client can connect.
func sessionListen(id string) (net.Listener, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	addr := ln.Addr().String()
	if err := os.WriteFile(sessionSock(id), []byte(addr), 0600); err != nil {
		ln.Close()
		return nil, fmt.Errorf("write session addr: %w", err)
	}
	return ln, nil
}

// sessionDial connects to the TCP address recorded in the session sock file.
func sessionDial(id string, timeout time.Duration) (net.Conn, error) {
	raw, err := os.ReadFile(sessionSock(id))
	if err != nil {
		return nil, fmt.Errorf("read session addr: %w", err)
	}
	return net.DialTimeout("tcp", strings.TrimSpace(string(raw)), timeout)
}

// sessionIPCReady returns true when the address file exists and is accepting connections.
func sessionIPCReady(id string) bool {
	raw, err := os.ReadFile(sessionSock(id))
	if err != nil {
		return false
	}
	conn, err := net.DialTimeout("tcp", strings.TrimSpace(string(raw)), 200*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// sessionIPCCleanup removes the address file.
func sessionIPCCleanup(id string) { _ = os.Remove(sessionSock(id)) }
