//go:build !windows

package platform

import "errors"

func PPGitProcesses() ([]Process, error) {
	return nil, errors.New("cross-installation stop is supported only on Windows; use the original installation's stop script")
}
