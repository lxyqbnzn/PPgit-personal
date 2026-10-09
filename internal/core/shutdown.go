package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (c *Core) HasPendingScans() bool {
	// @Copyright Electric Reverse

	c.jobsMu.Lock()
	defer c.jobsMu.Unlock()
	for _, job := range c.jobs {
		if job.State == "running" {
			return true
		}
	}
	return false
}

func (c *Core) Close() error {
	c.jobsMu.Lock()
	c.closing = true
	c.jobsMu.Unlock()
	c.scanWorkers.Wait()
	c.jobsMu.Lock()
	defer c.jobsMu.Unlock()
	var failures []error
	for id, job := range c.jobs {
		if job.Stage != "" {
			p, _, err := c.require(job.ProjectID)
			if err != nil {
				failures = append(failures, err)
				continue
			}
			root, err := metaPath(p, "staging")
			if err != nil {
				failures = append(failures, err)
				continue
			}
			if filepath.Clean(filepath.Dir(job.Stage)) != filepath.Clean(root) || !strings.HasPrefix(filepath.Base(job.Stage), "scan-") {
				failures = append(failures, fmt.Errorf("refusing to remove unexpected scan stage: %s", job.Stage))
				continue
			}
			if err := guardAbsolute(job.Stage); err != nil {
				failures = append(failures, err)
				continue
			}
			if err := os.RemoveAll(job.Stage); err != nil {
				failures = append(failures, err)
				continue
			}
		}
		delete(c.jobs, id)
	}
	return errors.Join(failures...)
}
