//go:build windows

package ssh

import "os"

// startWinchForwarder is a no-op on Windows: SIGWINCH does not exist.
func startWinchForwarder(_ chan os.Signal) {}
