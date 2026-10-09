package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func copyFile(source, destination string, mode os.FileMode, atomic bool) error {
	// @Copyright Electric Reverse

	if err := guardAbsolute(source); err != nil {
		return err
	}
	if err := guardAbsolute(destination); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	var out *os.File
	temp := destination
	if atomic {
		out, err = os.CreateTemp(filepath.Dir(destination), ".ppgit-copy-*")
		if err != nil {
			return err
		}
		temp = out.Name()
		defer os.Remove(temp)
	} else {
		out, err = os.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if err != nil {
			return err
		}
	}
	_, err = io.Copy(out, in)
	if err == nil {
		err = out.Sync()
	}
	closeErr := out.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Chmod(temp, mode); err != nil {
		return err
	}
	if atomic {
		return os.Rename(temp, destination)
	}
	return nil
}

func stageObject(p project, e Entry, destination string) error {
	source, err := objectPath(p, e.ObjectID)
	if err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() != e.Size {
		return errors.New("invalid checkout object size")
	}
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(out, hash), in)
	if err == nil {
		err = out.Sync()
	}
	closeErr := out.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if n != e.Size || hex.EncodeToString(hash.Sum(nil)) != e.ObjectID {
		return errors.New("checkout object integrity check failed")
	}
	return os.Chmod(destination, os.FileMode(e.Mode)&0777)
}

func cleanupEmptyParents(root, path string) {
	for path != root && path != filepath.Dir(path) {
		if err := os.Remove(path); err != nil {
			return
		}
		path = filepath.Dir(path)
	}
}

func hasWorkspacePath(p project, tree Snapshot, rel string) bool {
	// @Copyright Electric Reverse

	if _, ok := tree[rel]; ok {
		return true
	}
	for name := range tree {
		if strings.EqualFold(name, rel) {
			left, err := os.Lstat(filepath.Join(p.Path, filepath.FromSlash(name)))
			if err != nil {
				continue
			}
			right, err := os.Lstat(filepath.Join(p.Path, filepath.FromSlash(rel)))
			if err == nil && !isLink(left) && !isLink(right) && os.SameFile(left, right) {
				return true
			}
		}
	}
	return false
}

func preflightCheckout(p project, base, target Snapshot, changes []map[string]any) error {
	for _, change := range changes {
		rel := change["path"].(string)
		if _, err := safePath(p.Path, rel); err != nil {

			if !errors.Is(err, os.ErrNotExist) && !strings.Contains(strings.ToLower(err.Error()), "not a directory") {
				return err
			}
		}
		if _, needed := target[rel]; !needed {
			continue
		}
		parts := strings.Split(rel, "/")
		for i := 1; i < len(parts); i++ {
			parentRel := strings.Join(parts[:i], "/")
			path := filepath.Join(p.Path, filepath.FromSlash(parentRel))
			info, err := os.Lstat(path)
			if os.IsNotExist(err) {
				break
			}
			if err != nil {
				return err
			}
			if isLink(info) {
				return fmt.Errorf("checkout path is a link: %s", parentRel)
			}
			if !info.IsDir() {
				if !hasWorkspacePath(p, base, parentRel) {
					return fmt.Errorf("checkout would overwrite an unmanaged file: %s", parentRel)
				}
				if hasWorkspacePath(p, target, parentRel) {
					return fmt.Errorf("invalid file/directory transition: %s", parentRel)
				}
				break
			}
		}
		destination := filepath.Join(p.Path, filepath.FromSlash(rel))
		info, err := os.Lstat(destination)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "not a directory") {
				continue
			}
			return err
		}
		if isLink(info) {
			return fmt.Errorf("checkout path is a link: %s", rel)
		}
		if !info.IsDir() {
			if !hasWorkspacePath(p, base, rel) {
				return fmt.Errorf("checkout would overwrite an unmanaged file: %s", rel)
			}
			continue
		}
		err = filepath.WalkDir(destination, func(path string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			info, e := d.Info()
			if e != nil {
				return e
			}
			if isLink(info) {
				return fmt.Errorf("checkout directory contains a link: %s", path)
			}
			if d.IsDir() {
				return nil
			}
			r, e := filepath.Rel(p.Path, path)
			if e != nil {
				return e
			}
			r = filepath.ToSlash(r)
			if !hasWorkspacePath(p, base, r) {
				return fmt.Errorf("checkout would overwrite unmanaged content: %s", r)
			}
			if hasWorkspacePath(p, target, r) {
				return fmt.Errorf("checkout target conflicts with directory: %s", rel)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

type recoveryFile struct {
	Path     string `json:"path"`
	Backup   string `json:"backup,omitempty"`
	Prepared string `json:"prepared,omitempty"`
	Mode     uint32 `json:"mode"`
}
type recoveryRecord struct {
	Project    string         `json:"project"`
	OldHead    string         `json:"old_head"`
	TargetHead string         `json:"target_head"`
	Files      []recoveryFile `json:"files"`
}

type CheckoutConfirmationRequired struct {
	Token            string
	UncommittedCount int
}

func (e *CheckoutConfirmationRequired) Error() string {
	return "confirm discarding uncommitted workspace changes before switching versions"
}

var ErrCheckoutWorkspaceChanged = errors.New("workspace changed while preparing checkout; retry")

func checkoutConfirmationToken(p project, head, target version, workspace Snapshot) (string, error) {
	// @Copyright Electric Reverse

	raw, err := json.Marshal(struct {
		Project, Path, Head, Target, HeadTree, TargetTree string
		Workspace                                         Snapshot
	}{p.ID, p.Path, head.ID, target.ID, head.Tree, target.Tree, workspace})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func (c *Core) CheckoutVersion(id, versionID string) (result map[string]any, err error) {
	return c.CheckoutVersionConfirmed(id, versionID, "")
}

func (c *Core) CheckoutVersionConfirmed(id, versionID, confirmationToken string) (result map[string]any, err error) {
	p, l, err := c.require(id)
	if err != nil {
		return nil, err
	}
	l.Lock()
	defer l.Unlock()
	s, err := c.loadState(p)
	if err != nil {
		return nil, err
	}
	targetVersion, _, err := findVersion(s, versionID)
	if err != nil {
		return nil, err
	}
	head, _, err := findVersion(s, s.Head)
	if err != nil {
		return nil, err
	}
	tree, err := c.loadTree(p, head.Tree)
	if err != nil {
		return nil, err
	}
	rules, err := loadIgnores(p)
	if err != nil {
		return nil, err
	}
	base := filterTree(tree, rules)
	workspace, workspaceToken, _, err := c.scan(p, base, rules, false, "", nil)
	if err != nil {
		return nil, err
	}
	uncommittedCount := len(DiffSnapshots(base, workspace))
	if uncommittedCount != 0 {
		token, tokenErr := checkoutConfirmationToken(p, head, targetVersion, workspace)
		if tokenErr != nil {
			return nil, tokenErr
		}
		if confirmationToken != token {
			return nil, &CheckoutConfirmationRequired{Token: token, UncommittedCount: uncommittedCount}
		}
	}
	target, err := c.loadTree(p, targetVersion.Tree)
	if err != nil {
		return nil, err
	}

	base = workspace
	changes := DiffSnapshots(base, target)
	if err = preflightCheckout(p, base, target, changes); err != nil {
		return nil, err
	}
	stageRoot, err := metaPath(p, "checkout-staging")
	if err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(stageRoot, "restore-")
	if err != nil {
		return nil, err
	}
	preserve := false
	defer func() {
		if !preserve {
			os.RemoveAll(stage)
		}
	}()
	for _, name := range []string{"backup", "new"} {
		if err = os.Mkdir(filepath.Join(stage, name), 0700); err != nil {
			return nil, err
		}
	}
	record := recoveryRecord{Project: p.Path, OldHead: s.Head, TargetHead: versionID, Files: []recoveryFile{}}
	for i, change := range changes {
		rel := change["path"].(string)
		entry := recoveryFile{Path: rel}
		name := fmt.Sprintf("%08d", i)
		if old, ok := base[rel]; ok {
			entry.Backup = "backup/" + name
			entry.Mode = old.Mode
			source, e := safePath(p.Path, rel)
			if e != nil {
				return nil, e
			}
			if e = copyFile(source, filepath.Join(stage, filepath.FromSlash(entry.Backup)), os.FileMode(old.Mode), false); e != nil {
				return nil, e
			}
		}
		if next, ok := target[rel]; ok {
			entry.Prepared = "new/" + name
			if err = stageObject(p, next, filepath.Join(stage, filepath.FromSlash(entry.Prepared))); err != nil {
				return nil, err
			}
		}
		record.Files = append(record.Files, entry)
	}
	if err = writeJSON(filepath.Join(stage, "recovery.json"), record); err != nil {
		return nil, err
	}
	if c.checkoutHook != nil {
		if err = c.checkoutHook("before-recheck"); err != nil {
			return nil, err
		}
	}

	_, latestToken, err := workspaceFiles(p, rules)
	if err != nil {
		return nil, err
	}
	if latestToken != workspaceToken {
		return nil, ErrCheckoutWorkspaceChanged
	}
	if err = preflightCheckout(p, base, target, changes); err != nil {
		return nil, err
	}
	removed := []recoveryFile{}
	written := []recoveryFile{}

	rollback := func(cause error) error {
		var failures []error
		for i := len(written) - 1; i >= 0; i-- {
			path, e := safePath(p.Path, written[i].Path)
			if e != nil {
				failures = append(failures, e)
				continue
			}
			if e = os.Remove(path); e != nil && !os.IsNotExist(e) {
				failures = append(failures, e)
			}
			cleanupEmptyParents(p.Path, filepath.Dir(path))
		}
		for _, file := range record.Files {
			if file.Prepared != "" {
				cleanupEmptyParents(p.Path, filepath.Dir(filepath.Join(p.Path, filepath.FromSlash(file.Path))))
			}
		}
		for _, file := range removed {
			path, e := safePath(p.Path, file.Path)
			if e != nil {
				failures = append(failures, e)
				continue
			}
			if e = os.MkdirAll(filepath.Dir(path), 0755); e != nil {
				failures = append(failures, e)
				continue
			}
			if e = copyFile(filepath.Join(stage, filepath.FromSlash(file.Backup)), path, os.FileMode(file.Mode), true); e != nil {
				failures = append(failures, e)
			}
		}
		if len(failures) > 0 {
			preserve = true
			return fmt.Errorf("checkout failed (%v); rollback incomplete (%v); recovery data retained at %s", cause, errors.Join(failures...), stage)
		}
		return cause
	}
	deletions := append([]recoveryFile{}, record.Files...)
	sort.Slice(deletions, func(i, j int) bool {
		return strings.Count(deletions[i].Path, "/") > strings.Count(deletions[j].Path, "/")
	})
	for _, file := range deletions {
		if file.Backup == "" {
			continue
		}
		path, e := safePath(p.Path, file.Path)
		if e != nil {
			return nil, rollback(e)
		}
		if e = os.Remove(path); e != nil {
			return nil, rollback(e)
		}
		removed = append(removed, file)
		cleanupEmptyParents(p.Path, filepath.Dir(path))
	}
	if c.checkoutHook != nil {
		if e := c.checkoutHook("after-delete"); e != nil {
			return nil, rollback(e)
		}
	}
	for _, file := range record.Files {
		if file.Prepared == "" {
			continue
		}
		path, e := safePath(p.Path, file.Path)
		if e != nil {
			return nil, rollback(e)
		}
		if info, e := os.Stat(path); e == nil && info.IsDir() {
			if e = os.Remove(path); e != nil {
				return nil, rollback(e)
			}
		}
		if e = os.MkdirAll(filepath.Dir(path), 0755); e != nil {
			return nil, rollback(e)
		}
		if e = os.Rename(filepath.Join(stage, filepath.FromSlash(file.Prepared)), path); e != nil {
			return nil, rollback(e)
		}
		written = append(written, file)
	}
	if c.checkoutHook != nil {
		if e := c.checkoutHook("after-write"); e != nil {
			return nil, rollback(e)
		}
	}
	s.Head = versionID
	if err = c.saveState(p, s); err != nil {
		return nil, rollback(err)
	}
	c.discardProjectScans(id)
	return map[string]any{"project_id": p.ID, "name": p.Name, "path": p.Path, "head_version_id": s.Head, "current_version_id": s.Head, "short_version_id": s.Head[:8], "version_count": len(s.Versions), "current_version_message": targetVersion.Message}, nil
}
