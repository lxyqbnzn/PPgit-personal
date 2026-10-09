//go:build !windows

package platform

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func IsAddressInUse(err error) bool { return errors.Is(err, syscall.EADDRINUSE) }

func hideWindow(cmd *exec.Cmd) {}

func Lock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("data directory already in use or lock unavailable: %w", err)
	}
	return func() { _ = f.Close() }, nil
}
