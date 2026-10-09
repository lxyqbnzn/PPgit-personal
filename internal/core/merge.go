package core

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"unicode/utf8"
)

var ErrMergeConflict = errors.New("version file changed since comparison; refresh before confirming")
var ErrVersionReadOnly = errors.New("this version is read-only")

func (c *Core) UpdateVersionFile(projectID, fromID, toID, path, expectedFrom, expectedTo string, transform func([]byte, []byte) ([]byte, error)) (map[string]any, error) {
	if err := ValidatePath(path); err != nil {
		return nil, err
	}
	if (expectedFrom != "" && !storageID.MatchString(expectedFrom)) || (expectedTo != "" && !storageID.MatchString(expectedTo)) {
		return nil, errors.New("invalid expected object identifier")
	}
	p, lock, err := c.require(projectID)
	if err != nil {
		return nil, err
	}
	lock.Lock()
	defer lock.Unlock()
	s, err := c.loadState(p)
	if err != nil {
		return nil, err
	}
	from, fromIndex, err := findVersion(s, fromID)
	if err != nil {
		return nil, err
	}
	to, toIndex, err := findVersion(s, toID)
	if err != nil {
		return nil, err
	}
	if fromIndex >= toIndex {
		return nil, errors.New("select distinct versions in oldest-to-newest order")
	}
	if to.ReadOnlyReason != "" {
		return nil, ErrVersionReadOnly
	}
	left, err := c.loadTree(p, from.Tree)
	if err != nil {
		return nil, err
	}
	right, err := c.loadTree(p, to.Tree)
	if err != nil {
		return nil, err
	}
	leftEntry, hasLeft := left[path]
	rightEntry, hasRight := right[path]
	if leftEntry.ObjectID != expectedFrom || rightEntry.ObjectID != expectedTo {
		return nil, ErrMergeConflict
	}
	if !hasLeft && !hasRight {
		return nil, errors.New("file is absent from both versions")
	}

	read := func(entry Entry, exists bool) ([]byte, error) {
		if !exists {
			return nil, nil
		}
		if !entry.IsText {
			return nil, errors.New("only UTF-8 text files can be merged")
		}
		raw, err := loadObject(p, entry.ObjectID, MaxPreviewSize)
		if err != nil {
			return nil, err
		}
		if !utf8.Valid(raw) {
			return nil, errors.New("only UTF-8 text files can be merged")
		}
		return raw, nil
	}
	oldBytes, err := read(leftEntry, hasLeft)
	if err != nil {
		return nil, err
	}
	newBytes, err := read(rightEntry, hasRight)
	if err != nil {
		return nil, err
	}
	merged, err := transform(oldBytes, newBytes)
	if err != nil {
		return nil, err
	}
	if int64(len(merged)) > MaxPreviewSize || !utf8.Valid(merged) {
		return nil, errors.New("merged text exceeds the supported limit")
	}
	sum := sha256.Sum256(merged)
	objectID := hex.EncodeToString(sum[:])
	if hasRight && objectID == rightEntry.ObjectID {
		return nil, errors.New("the selected rows do not change the target file")
	}
	next := make(Snapshot, len(right)+1)
	for name, entry := range right {
		next[name] = entry
	}
	entry := rightEntry
	if !hasRight {
		entry = leftEntry
	}
	entry.ObjectID, entry.Size = objectID, int64(len(merged))

	entry.Signature, entry.ModTime = "", 0
	next[path] = entry
	if err := validateSnapshot(next); err != nil {
		return nil, err
	}
	parent := Snapshot{}
	if to.Parent != "" {
		v, _, err := findVersion(s, to.Parent)
		if err != nil {
			return nil, err
		}
		parent, err = c.loadTree(p, v.Tree)
		if err != nil {
			return nil, err
		}
	}
	to.Changed = len(DiffSnapshots(parent, next))
	for index, child := range s.Versions {
		if child.Parent != to.ID {
			continue
		}
		childTree, err := c.loadTree(p, child.Tree)
		if err != nil {
			return nil, err
		}
		s.Versions[index].Changed = len(DiffSnapshots(next, childTree))
	}
	destination, err := objectPath(p, objectID)
	if err != nil {
		return nil, err
	}
	objectMade, treeMade, committed := false, false, false
	treeID := ""
	defer func() {
		if !committed {
			if objectMade {
				_ = os.Remove(destination)
			}
			if treeMade {
				if path, err := metaPath(p, "trees", treeID+".json"); err == nil {
					_ = os.Remove(path)
				}
			}
		}
	}()
	if _, err = os.Stat(destination); os.IsNotExist(err) {
		if err = os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
			return nil, err
		}
		if err = atomicWrite(destination, merged, 0600); err != nil {
			return nil, err
		}
		objectMade = true
	} else if err != nil {
		return nil, err
	} else if err = verifyObjectFile(destination, objectID, int64(len(merged))); err != nil {
		return nil, err
	}
	treeID, treeMade, err = c.storeTree(p, next)
	if err != nil {
		return nil, err
	}
	to.Tree = treeID
	s.Versions[toIndex] = to
	if err = c.saveState(p, s); err != nil {
		return nil, err
	}
	committed = true
	c.discardProjectScans(projectID)
	return map[string]any{"ok": true, "version": serializeVersion(to, toIndex), "path": path, "object_id": objectID}, nil
}

func (c *Core) ProtectVersions(projectID string, ids []string, reason string) error {
	if reason == "" {
		return errors.New("a protection reason is required")
	}
	p, lock, err := c.require(projectID)
	if err != nil {
		return err
	}
	lock.Lock()
	defer lock.Unlock()
	s, err := c.loadState(p)
	if err != nil {
		return err
	}
	wanted := map[string]bool{}
	for _, id := range ids {
		if _, _, err := findVersion(s, id); err != nil {
			return err
		}
		wanted[id] = true
	}
	changed := false
	for i, v := range s.Versions {
		if (ids == nil || wanted[v.ID]) && v.ReadOnlyReason == "" {
			s.Versions[i].ReadOnlyReason = reason
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return c.saveState(p, s)
}
