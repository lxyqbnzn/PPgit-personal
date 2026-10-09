package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (c *Core) ListProjects() ([]map[string]any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]map[string]any, 0, len(c.projects))
	for _, p := range c.projects {
		out = append(out, map[string]any{"project_id": p.ID, "name": p.Name, "path": p.Path})
	}
	return out, nil
}

func (c *Core) AddProject(path string) (map[string]any, error) {
	abs, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return nil, err
	}
	if err = guardAbsolute(abs); err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("project path must be a directory")
	}
	c.mu.Lock()
	for _, p := range c.projects {
		same := p.Path == abs
		if runtime.GOOS == "windows" {
			same = strings.EqualFold(p.Path, abs)
		}
		if same {
			c.mu.Unlock()
			return c.GetProject(p.ID)
		}
	}
	p := project{newID()[:8], filepath.Base(abs), abs}
	if err = c.initRepo(p); err != nil {
		c.mu.Unlock()
		return nil, err
	}
	updated := append(append([]project{}, c.projects...), p)
	if err = writeJSON(filepath.Join(c.dataDir, "config.json"), config{updated}); err != nil {
		c.mu.Unlock()
		return nil, err
	}
	c.projects = updated
	c.mu.Unlock()
	return c.GetProject(p.ID)
}

func (c *Core) initRepo(p project) error {
	// @Copyright Electric Reverse

	statePath, err := metaPath(p, "state.json")
	if err != nil {
		return err
	}
	if _, err = os.Stat(statePath); err == nil {
		_, err = c.loadState(p)
		return err
	} else if !os.IsNotExist(err) {
		return err
	}
	legacy, err := metaPath(p, "versions.jsonl")
	if err != nil {
		return err
	}
	if _, err = os.Stat(legacy); err == nil {
		return errors.New("this directory contains a Python repository; use a fresh project directory for the Go edition")
	} else if !os.IsNotExist(err) {
		return err
	}
	for _, dir := range []string{"objects", "trees", "staging", "checkout-staging"} {
		path, err := metaPath(p, dir)
		if err != nil {
			return err
		}
		if err = os.MkdirAll(path, 0700); err != nil {
			return err
		}
	}
	ignore, err := safePath(p.Path, ".ppgitignore")
	if err != nil {
		return err
	}
	if _, err = os.Stat(ignore); os.IsNotExist(err) {
		if err = atomicWrite(ignore, []byte("# PPGit ignore rules\n# Examples:\n# build/\n# *.log\n"), 0644); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	rules, err := loadIgnores(p)
	if err != nil {
		return err
	}
	snapshot, _, objects, err := c.scan(p, nil, rules, true, "", nil)
	if err != nil {
		c.cleanupObjects(p, objects)
		return err
	}
	committed := false
	var tree string
	var made bool
	defer func() {
		if !committed {
			c.cleanupObjects(p, objects)
			if made {
				if path, e := metaPath(p, "trees", tree+".json"); e == nil {
					os.Remove(path)
				}
			}
		}
	}()
	tree, made, err = c.storeTree(p, snapshot)
	if err != nil {
		return err
	}
	v := version{ID: newID(), Message: "initial", Created: time.Now().UTC().Format(time.RFC3339Nano), Tree: tree, Changed: len(snapshot)}
	if err = c.saveState(p, state{formatVersion, v.ID, []version{v}}); err != nil {
		return err
	}
	committed = true
	return nil
}

func (c *Core) cleanupObjects(p project, ids []string) {
	for _, id := range ids {
		if path, err := objectPath(p, id); err == nil {
			os.Remove(path)
		}
	}
}

func (c *Core) GetProject(id string) (map[string]any, error) {
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
	if err = c.cleanupStaging(p); err != nil {
		return nil, err
	}
	head, _, err := findVersion(s, s.Head)
	if err != nil {
		return nil, err
	}
	next := len(s.Versions)
	for _, v := range s.Versions {
		msg := strings.ToUpper(strings.TrimSpace(v.Message))
		if strings.HasPrefix(msg, "V") {
			n, e := strconv.Atoi(msg[1:])
			if e == nil && n >= next {
				next = n + 1
			}
		}
	}
	return map[string]any{"project_id": p.ID, "name": p.Name, "path": p.Path, "head_version_id": s.Head, "current_version_id": s.Head, "short_version_id": s.Head[:8], "version_count": len(s.Versions), "current_version_message": head.Message, "next_version_message": fmt.Sprintf("V%d", next)}, nil
}

func (c *Core) RemoveProject(id string) error {
	// @Copyright Electric Reverse

	_, l, err := c.require(id)
	if err != nil {
		return err
	}
	l.Lock()
	defer l.Unlock()
	c.mu.Lock()
	updated := make([]project, 0, len(c.projects))
	for _, p := range c.projects {
		if p.ID != id {
			updated = append(updated, p)
		}
	}
	err = writeJSON(filepath.Join(c.dataDir, "config.json"), config{updated})
	if err == nil {
		c.projects = updated
	}
	c.mu.Unlock()
	if err == nil {
		c.discardProjectScans(id)
	}
	return err
}

func (c *Core) GetHistory(id string, page, perPage int) (map[string]any, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 50
	}
	if perPage > 200 {
		perPage = 200
	}
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
	total := len(s.Versions)
	items := []map[string]any{}
	if page <= ((total + perPage - 1) / perPage) {
		for i := total - 1 - (page-1)*perPage; i >= 0 && len(items) < perPage; i-- {
			items = append(items, serializeVersion(s.Versions[i], i))
		}
	}
	return map[string]any{"items": items, "page": page, "per_page": perPage, "total": total, "pages": (total + perPage - 1) / perPage}, nil
}

func (c *Core) versionDetail(p project, s state, id string) (map[string]any, error) {
	v, seq, err := findVersion(s, id)
	if err != nil {
		return nil, err
	}
	tree, err := c.loadTree(p, v.Tree)
	if err != nil {
		return nil, err
	}
	base := Snapshot{}
	if v.Parent != "" {
		parent, _, err := findVersion(s, v.Parent)
		if err != nil {
			return nil, err
		}
		base, err = c.loadTree(p, parent.Tree)
		if err != nil {
			return nil, err
		}
	}
	result := serializeVersion(v, seq)
	result["files"] = DiffSnapshots(base, tree)
	return result, nil
}

func (c *Core) GetVersionDetail(id, versionID string) (map[string]any, error) {
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
	return c.versionDetail(p, s, versionID)
}

func (c *Core) CreateVersion(id, message, scanID string) (result map[string]any, err error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return nil, errors.New("version message must not be empty")
	}
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
	var snapshot Snapshot
	var stage string
	objects := []string{}
	treeID := ""
	treeMade := false
	committed := false
	defer func() {
		if !committed {
			c.cleanupObjects(p, objects)
			if treeMade {
				if path, e := metaPath(p, "trees", treeID+".json"); e == nil {
					os.Remove(path)
				}
			}
		}
	}()
	if scanID != "" {
		c.jobsMu.Lock()
		j := c.jobs[scanID]
		if j != nil && j.ProjectID == id && j.State == "completed" && j.Base == s.Head && j.Snapshot != nil && time.Since(j.Access) < 10*time.Minute {
			_, token, e := workspaceFiles(p, rules)
			if e != nil {
				c.jobsMu.Unlock()
				return nil, e
			}
			if token == j.Token {
				snapshot = j.Snapshot
				stage = j.Stage
				j.Access = time.Now()
			}
		}
		if snapshot != nil && stage != "" {
			sizes := make(map[string]int64, len(j.Objects))
			for _, entry := range snapshot {
				sizes[entry.ObjectID] = entry.Size
			}
			seen := map[string]bool{}
			for _, name := range j.Objects {
				if seen[name] {
					continue
				}
				seen[name] = true
				if !storageID.MatchString(name) {
					c.jobsMu.Unlock()
					return nil, errors.New("invalid staged object")
				}
				src := filepath.Join(stage, name)
				if e := verifyObjectFile(src, name, sizes[name]); e != nil {
					c.jobsMu.Unlock()
					return nil, e
				}
				dst, e := objectPath(p, name)
				if e != nil {
					c.jobsMu.Unlock()
					return nil, e
				}
				if _, e = os.Stat(dst); e == nil {
					continue
				} else if !os.IsNotExist(e) {
					c.jobsMu.Unlock()
					return nil, e
				}
				if e = os.MkdirAll(filepath.Dir(dst), 0700); e != nil {
					c.jobsMu.Unlock()
					return nil, e
				}
				if e = copyFile(src, dst, 0600, true); e != nil {
					c.jobsMu.Unlock()
					return nil, e
				}
				objects = append(objects, name)
				if e = verifyObjectFile(dst, name, sizes[name]); e != nil {
					c.jobsMu.Unlock()
					return nil, e
				}
			}
		}
		c.jobsMu.Unlock()
	}
	if snapshot == nil {
		snapshot, _, objects, err = c.scan(p, base, rules, true, "", nil)
		if err != nil {
			return nil, err
		}
	}
	changes := DiffSnapshots(base, snapshot)
	if len(changes) == 0 {
		return nil, errors.New("there are no changes to save")
	}
	treeID, treeMade, err = c.storeTree(p, snapshot)
	if err != nil {
		return nil, err
	}
	v := version{ID: newID(), Parent: s.Head, Message: message, Created: time.Now().UTC().Format(time.RFC3339Nano), Tree: treeID, Changed: len(changes)}
	s.Head = v.ID
	s.Versions = append(s.Versions, v)
	if err = c.saveState(p, s); err != nil {
		return nil, err
	}
	committed = true
	c.discardProjectScans(id)
	return c.versionDetail(p, s, v.ID)
}

func (c *Core) DeleteVersion(id, versionID string) (map[string]any, error) {
	return c.deleteVersion(id, versionID, true)
}

func (c *Core) DeleteVersionRecord(id, versionID string) (map[string]any, error) {
	// @Copyright Electric Reverse

	return c.deleteVersion(id, versionID, false)
}

func (c *Core) deleteVersion(id, versionID string, cleanup bool) (map[string]any, error) {
	// @Copyright Electric Reverse

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
	v, _, err := findVersion(s, versionID)
	if err != nil {
		return nil, err
	}
	if len(s.Versions) == 1 {
		return nil, errors.New("cannot delete the last version")
	}
	updated := make([]version, 0, len(s.Versions)-1)
	for _, item := range s.Versions {
		if item.ID == versionID {
			continue
		}
		if item.Parent == versionID {
			item.Parent = v.Parent
			base := Snapshot{}
			if v.Parent != "" {
				parent, _, e := findVersion(s, v.Parent)
				if e != nil {
					return nil, e
				}
				base, e = c.loadTree(p, parent.Tree)
				if e != nil {
					return nil, e
				}
			}
			target, e := c.loadTree(p, item.Tree)
			if e != nil {
				return nil, e
			}
			item.Changed = len(DiffSnapshots(base, target))
		}
		updated = append(updated, item)
	}
	s.Versions = updated
	if s.Head == versionID {
		s.Head = v.Parent
		if s.Head == "" {
			s.Head = updated[len(updated)-1].ID
		}
	}
	if err = c.saveState(p, s); err != nil {
		return nil, err
	}
	c.discardProjectScans(id)
	result := map[string]any{"deleted": true, "deleted_objects": 0, "deleted_trees": 0, "freed_bytes": int64(0)}
	if cleanup {
		stats, e := c.gc(p, s)
		if e != nil {
			result["cleanup_warning"] = e.Error()
		} else {
			for key, value := range stats {
				result[key] = value
			}
		}
	}
	return result, nil
}

func (c *Core) GetFileContent(id, versionID, file string) (map[string]any, error) {
	if err := ValidatePath(file); err != nil {
		return nil, err
	}
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
	v, _, err := findVersion(s, versionID)
	if err != nil {
		return nil, err
	}
	tree, err := c.loadTree(p, v.Tree)
	if err != nil {
		return nil, err
	}
	e, ok := tree[file]
	if !ok {
		return nil, errors.New("file does not exist")
	}
	out := map[string]any{"path": file, "content": nil, "too_large": false, "message": nil, "size": e.Size, "mime_type": e.MIMEType}
	if e.Size > MaxPreviewSize {
		out["too_large"] = true
		out["message"] = "[File exceeds 10 MiB preview limit]"
		return out, nil
	}
	if !e.IsText {
		out["message"] = "[Binary file cannot be previewed]"
		return out, nil
	}
	raw, err := loadObject(p, e.ObjectID, MaxPreviewSize)
	if err != nil {
		return nil, err
	}
	out["content"] = strings.ToValidUTF8(string(raw), "\ufffd")
	return out, nil
}

type treeNode struct {
	Name, Path         string
	Directory, Deleted bool
	Size               int64
	Children           map[string]*treeNode
}

func addTreeNode(root map[string]*treeNode, path string, e Entry, deleted bool) {
	parts := strings.Split(path, "/")
	cursor := root
	for i, part := range parts {
		last := i == len(parts)-1
		if last {
			if _, exists := cursor[part]; !exists {
				cursor[part] = &treeNode{Name: part, Path: path, Size: e.Size, Deleted: deleted}
			}
			return
		}
		node := cursor[part]
		if node != nil && !node.Directory {
			return
		}
		if node == nil {
			node = &treeNode{Name: part, Path: strings.Join(parts[:i+1], "/"), Directory: true, Children: map[string]*treeNode{}}
			cursor[part] = node
		}
		cursor = node.Children
	}
}

func serializeNodes(root map[string]*treeNode) []map[string]any {
	keys := make([]string, 0, len(root))
	for key := range root {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		node := root[key]
		item := map[string]any{"name": node.Name, "path": node.Path, "type": "file", "size": node.Size}
		if node.Directory {
			item["type"] = "directory"
			delete(item, "size")
			item["children"] = serializeNodes(node.Children)
		}
		if node.Deleted {
			item["deleted"] = true
		}
		out = append(out, item)
	}
	return out
}

func (c *Core) GetFileTree(id, versionID string) ([]map[string]any, error) {
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
	v, _, err := findVersion(s, versionID)
	if err != nil {
		return nil, err
	}
	tree, err := c.loadTree(p, v.Tree)
	if err != nil {
		return nil, err
	}
	root := map[string]*treeNode{}
	for _, path := range sortedPaths(tree) {
		addTreeNode(root, path, tree[path], false)
	}
	if v.Parent != "" {
		parent, _, e := findVersion(s, v.Parent)
		if e != nil {
			return nil, e
		}
		base, e := c.loadTree(p, parent.Tree)
		if e != nil {
			return nil, e
		}
		for _, path := range sortedPaths(base) {
			if _, ok := tree[path]; !ok {
				addTreeNode(root, path, Entry{}, true)
			}
		}
	}
	return serializeNodes(root), nil
}
