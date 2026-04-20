//go:build !windows

package ssh

import (
	"os"
	"os/signal"
	"syscall"
)

func startWinchForwarder(sigCh chan os.Signal) {
	signal.Notify(sigCh, syscall.SIGWINCH)
}
