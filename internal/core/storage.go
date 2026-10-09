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
	"time"
)

func readJSON(p string, out any) error {
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	if err = dec.Decode(out); err != nil {
		return err
	}
	var extra any
	if err = dec.Decode(&extra); err != io.EOF {
		return errors.New("unexpected trailing JSON data")
	}
	return nil
}

func writeJSON(p string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return atomicWrite(p, raw, 0600)
}

func atomicWrite(p string, raw []byte, mode os.FileMode) error {
	if err := guardAbsolute(p); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(p), ".ppgit-write-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Chmod(name, mode); err != nil {
		return err
	}
	return os.Rename(name, p)
}

func (c *Core) loadState(p project) (state, error) {
	path, err := metaPath(p, "state.json")
	if err != nil {
		return state{}, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return state{}, err
	}
	sig, err := metadataSignature(path, info)
	if err != nil {
		return state{}, err
	}
	c.cacheMu.Lock()
	cached, ok := c.states[path]
	if ok && cached.Signature == sig {
		cached.Access = time.Now()
		c.states[path] = cached
		c.cacheMu.Unlock()
		s := cached.State
		s.Versions = append([]version{}, s.Versions...)
		return s, nil
	}
	c.cacheMu.Unlock()
	var s state
	if err = readJSON(path, &s); err != nil {
		return s, fmt.Errorf("invalid repository state: %w", err)
	}
	if s.Format != formatVersion || len(s.Versions) == 0 || !versionID.MatchString(s.Head) {
		return s, errors.New("invalid or unsupported repository state")
	}
	ids := map[string]bool{}
	for _, v := range s.Versions {
		if !versionID.MatchString(v.ID) || !storageID.MatchString(v.Tree) || ids[v.ID] || v.Changed < 0 {
			return s, errors.New("invalid version index")
		}
		if v.Parent != "" && !ids[v.Parent] {
			return s, errors.New("invalid version ancestry")
		}
		ids[v.ID] = true
	}
	if !ids[s.Head] {
		return s, errors.New("repository HEAD is missing")
	}
	c.cacheMu.Lock()
	c.states[path] = cacheState{sig, s, time.Now()}
	if len(c.states) > 32 {
		var oldest string
		var age time.Time
		for k, v := range c.states {
			if oldest == "" || v.Access.Before(age) {
				oldest, age = k, v.Access
			}
		}
		delete(c.states, oldest)
	}
	c.cacheMu.Unlock()
	s.Versions = append([]version{}, s.Versions...)
	return s, nil
}

func (c *Core) saveState(p project, s state) error {
	path, err := metaPath(p, "state.json")
	if err != nil {
		return err
	}
	if err = writeJSON(path, s); err != nil {
		return err
	}
	c.cacheMu.Lock()
	delete(c.states, path)
	c.cacheMu.Unlock()
	return nil
}

func (c *Core) loadTree(p project, id string) (Snapshot, error) {
	// @Copyright Electric Reverse

	if !storageID.MatchString(id) {
		return nil, errors.New("invalid tree identifier")
	}
	path, err := metaPath(p, "trees", id+".json")
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("missing tree: %w", err)
	}
	sig, err := metadataSignature(path, info)
	if err != nil {
		return nil, err
	}
	c.cacheMu.Lock()
	cached, ok := c.trees[path]
	if ok && cached.Signature == sig {
		cached.Access = time.Now()
		c.trees[path] = cached
		c.cacheMu.Unlock()
		return cached.Snapshot, nil
	}
	c.cacheMu.Unlock()
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != id {
		return nil, errors.New("tree integrity check failed")
	}
	var s Snapshot
	if err = json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	if err = validateSnapshot(s); err != nil {
		return nil, err
	}
	if len(s) <= 200000 {
		c.cacheMu.Lock()
		c.trees[path] = cacheTree{sig, s, time.Now()}
		total := 0
		for _, t := range c.trees {
			total += len(t.Snapshot)
		}
		for len(c.trees) > 32 || total > 200000 {
			var oldest string
			var age time.Time
			for k, v := range c.trees {
				if oldest == "" || v.Access.Before(age) {
					oldest, age = k, v.Access
				}
			}
			total -= len(c.trees[oldest].Snapshot)
			delete(c.trees, oldest)
		}
		c.cacheMu.Unlock()
	}
	return s, nil
}

func (c *Core) storeTree(p project, s Snapshot) (string, bool, error) {
	if err := validateSnapshot(s); err != nil {
		return "", false, err
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return "", false, err
	}
	sum := sha256.Sum256(raw)
	id := hex.EncodeToString(sum[:])
	path, err := metaPath(p, "trees", id+".json")
	if err != nil {
		return "", false, err
	}
	if _, err = os.Stat(path); err == nil {
		existing, e := os.ReadFile(path)
		if e != nil {
			return "", false, e
		}
		hash := sha256.Sum256(existing)
		if hash == sum {
			return id, false, nil
		}
	} else if !os.IsNotExist(err) {
		return "", false, err
	}
	if err = atomicWrite(path, raw, 0600); err != nil {
		return "", false, err
	}
	return id, true, nil
}

func objectPath(p project, id string) (string, error) {
	if !storageID.MatchString(id) {
		return "", errors.New("invalid object identifier")
	}
	return metaPath(p, "objects", id[:2], id[2:])
}

func (c *Core) ReadVersionSnapshot(projectID, versionID string) (map[string]any, Snapshot, error) {
	p, l, err := c.require(projectID)
	if err != nil {
		return nil, nil, err
	}
	l.Lock()
	defer l.Unlock()
	s, err := c.loadState(p)
	if err != nil {
		return nil, nil, err
	}
	v, seq, err := findVersion(s, versionID)
	if err != nil {
		return nil, nil, err
	}
	tree, err := c.loadTree(p, v.Tree)
	if err != nil {
		return nil, nil, err
	}
	copy := make(Snapshot, len(tree))
	for k, e := range tree {
		copy[k] = e
	}
	return serializeVersion(v, seq), copy, nil
}

func (c *Core) ReadObject(projectID, id string, maxBytes int64) ([]byte, error) {
	p, l, err := c.require(projectID)
	if err != nil {
		return nil, err
	}
	l.Lock()
	defer l.Unlock()
	return loadObject(p, id, maxBytes)
}

func loadObject(p project, id string, maxBytes int64) ([]byte, error) {
	path, err := objectPath(p, id)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if maxBytes > 0 && info.Size() > maxBytes {
		return nil, ErrTooLarge
	}
	var r io.Reader = f
	if maxBytes > 0 {
		r = io.LimitReader(f, maxBytes+1)
	}
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if maxBytes > 0 && int64(len(raw)) > maxBytes {
		return nil, ErrTooLarge
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != id {
		return nil, errors.New("object integrity check failed")
	}
	return raw, nil
}

func verifyObjectFile(path, id string, size int64) error {
	if err := guardAbsolute(path); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() != size {
		return errors.New("staged object size mismatch")
	}
	hash := sha256.New()
	n, err := io.Copy(hash, f)
	if err != nil {
		return err
	}
	if n != size || hex.EncodeToString(hash.Sum(nil)) != id {
		return errors.New("staged object integrity check failed")
	}
	return nil
}

func (c *Core) gc(p project, s state) (map[string]any, error) {
	treeIDs := map[string]bool{}
	objects := map[string]bool{}
	for _, v := range s.Versions {
		if treeIDs[v.Tree] {
			continue
		}
		treeIDs[v.Tree] = true
		tree, err := c.loadTree(p, v.Tree)
		if err != nil {
			return nil, err
		}
		for _, e := range tree {
			objects[e.ObjectID] = true
		}
	}
	result := map[string]any{"deleted_objects": 0, "deleted_trees": 0, "freed_bytes": int64(0)}
	for _, kind := range []string{"objects", "trees"} {
		root, err := metaPath(p, kind)
		if err != nil {
			return nil, err
		}
		err = filepath.WalkDir(root, func(path string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			info, e := d.Info()
			if e != nil {
				return e
			}
			if isLink(info) {
				return fmt.Errorf("unsafe storage link: %s", path)
			}
			if d.IsDir() {
				return nil
			}
			rel, e := filepath.Rel(root, path)
			if e != nil {
				return e
			}
			id := filepath.ToSlash(rel)
			keep := false
			if kind == "trees" {
				if len(id) < 5 {
					return errors.New("invalid tree storage name")
				}
				id = id[:len(id)-5]
				keep = treeIDs[id]
			} else {
				id = filepath.Base(filepath.Dir(path)) + d.Name()
				keep = objects[id]
			}
			if !storageID.MatchString(id) {
				return fmt.Errorf("invalid storage object name: %s", rel)
			}
			if !keep {
				if e = os.Remove(path); e != nil {
					return e
				}
				key := "deleted_" + kind
				result[key] = result[key].(int) + 1
				result["freed_bytes"] = result["freed_bytes"].(int64) + info.Size()
			}
			return nil
		})
		if err != nil {
			return result, err
		}
	}
	c.cacheMu.Lock()
	c.trees = map[string]cacheTree{}
	c.cacheMu.Unlock()
	return result, nil
}

func sortedPaths(s Snapshot) []string {
	out := make([]string, 0, len(s))
	for p := range s {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func (c *Core) cleanupStaging(p project) error {
	c.jobsMu.Lock()
	active := map[string]bool{}
	for _, job := range c.jobs {
		if job.Stage != "" {
			active[job.Stage] = true
		}
	}
	c.jobsMu.Unlock()
	cutoff := time.Now().Add(-24 * time.Hour)
	for _, kind := range []string{"staging", "checkout-staging"} {
		root, err := metaPath(p, kind)
		if err != nil {
			return err
		}
		entries, err := os.ReadDir(root)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			path := filepath.Join(root, entry.Name())
			if active[path] {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if isLink(info) {
				return fmt.Errorf("unsafe staging link: %s", path)
			}
			if !info.ModTime().Before(cutoff) {
				continue
			}
			if info.IsDir() {
				if _, err = os.Lstat(filepath.Join(path, "recovery.json")); err == nil {
					continue
				} else if !os.IsNotExist(err) {
					return err
				}
			}
			if err = guardAbsolute(path); err != nil {
				return err
			}
			if err = filepath.WalkDir(path, func(current string, d os.DirEntry, e error) error {
				if e != nil {
					return e
				}
				i, e := d.Info()
				if e != nil {
					return e
				}
				if isLink(i) {
					return fmt.Errorf("unsafe staging link: %s", current)
				}
				return nil
			}); err != nil {
				return err
			}
			if err = os.RemoveAll(path); err != nil {
				return err
			}
		}
	}
	return nil
}
