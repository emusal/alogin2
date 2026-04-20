//go:build !windows

package cli

import (
	"net"
	"os"
	"time"
)

// sessionListen creates a Unix domain socket for the session bridge.
func sessionListen(id string) (net.Listener, error) {
	p := sessionSock(id)
	_ = os.Remove(p)
	ln, err := net.Listen("unix", p)
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(p, 0600)
	return ln, nil
}

// sessionDial connects to the session bridge socket.
func sessionDial(id string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("unix", sessionSock(id), timeout)
}

// sessionIPCReady returns true when the socket file exists and is accepting connections.
func sessionIPCReady(id string) bool {
	if _, err := os.Stat(sessionSock(id)); err != nil {
		return false
	}
	conn, err := net.DialTimeout("unix", sessionSock(id), 200*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// sessionIPCCleanup removes the socket file.
func sessionIPCCleanup(id string) { _ = os.Remove(sessionSock(id)) }
