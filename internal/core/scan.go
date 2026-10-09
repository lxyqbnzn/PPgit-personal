package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var defaultIgnores = []string{".ppgit/", ".git/", "__pycache__/", "node_modules/", ".env", ".venv/", "venv/", ".idea/", ".vscode/", "*.pyc"}
var copyBuffers = sync.Pool{New: func() any { b := make([]byte, 128*1024); return &b }}

type ignoreRule struct {
	Negative, Directory, Anchored, Slash, Magic bool
	Pattern                                     string
	RX                                          *regexp.Regexp
}

func compileGlob(glob string) *regexp.Regexp {
	// @Copyright Electric Reverse

	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(glob); i++ {
		switch glob[i] {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '[':
			end := strings.IndexByte(glob[i+1:], ']')
			if end < 0 {
				b.WriteString(`\[`)
				continue
			}
			part := glob[i+1 : i+1+end]
			if strings.HasPrefix(part, "!") {
				part = "^" + part[1:]
			}
			b.WriteByte('[')
			b.WriteString(part)
			b.WriteByte(']')
			i += end + 1
		default:
			b.WriteString(regexp.QuoteMeta(string(glob[i])))
		}
	}
	b.WriteString("$")
	r, err := regexp.Compile(b.String())
	if err != nil {
		return regexp.MustCompile("^" + regexp.QuoteMeta(glob) + "$")
	}
	return r
}

func loadIgnores(p project) ([]ignoreRule, error) {
	file, err := safePath(p.Path, ".ppgitignore")
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	patterns := append([]string{}, defaultIgnores...)
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		if line != "" && !strings.HasPrefix(line, "#") {
			patterns = append(patterns, line)
		}
	}
	rules := make([]ignoreRule, 0, len(patterns))
	for _, pattern := range patterns {
		r := ignoreRule{}
		if strings.HasPrefix(pattern, "!") {
			r.Negative = true
			pattern = pattern[1:]
		}
		pattern = strings.TrimSpace(strings.ReplaceAll(pattern, "\\", "/"))
		pattern = strings.TrimPrefix(pattern, "./")
		r.Directory = strings.HasSuffix(pattern, "/")
		r.Anchored = strings.HasPrefix(pattern, "/")
		pattern = strings.Trim(pattern, "/")
		if pattern == "" {
			continue
		}
		r.Pattern = pattern
		r.Slash = strings.Contains(pattern, "/")
		r.Magic = strings.ContainsAny(pattern, "*?[")
		r.RX = compileGlob(pattern)
		rules = append(rules, r)
	}
	return rules, nil
}

func (r ignoreRule) matches(p string) bool {
	parts := strings.Split(p, "/")
	if r.Directory {
		if !r.Magic && !r.Slash {
			if r.Anchored {
				return parts[0] == r.Pattern
			}
			for _, part := range parts {
				if part == r.Pattern {
					return true
				}
			}
			return false
		}
		if r.Anchored {
			for i := range parts {
				if r.RX.MatchString(strings.Join(parts[:i+1], "/")) {
					return true
				}
			}
			return false
		}
		for start := range parts {
			for end := start + 1; end <= len(parts); end++ {
				if r.RX.MatchString(strings.Join(parts[start:end], "/")) {
					return true
				}
			}
		}
		return false
	}
	if !r.Slash {
		for i, part := range parts {
			if r.Anchored && i > 0 {
				break
			}
			if !r.Magic && part == r.Pattern || r.Magic && r.RX.MatchString(part) {
				return true
			}
		}
		return false
	}
	for i := range parts {
		if r.Anchored && i > 0 {
			break
		}
		candidate := strings.Join(parts[i:], "/")
		if r.RX.MatchString(candidate) || !r.Magic && strings.HasPrefix(candidate, r.Pattern+"/") {
			return true
		}
	}
	return false
}

func ignored(p string, rules []ignoreRule) bool {
	for _, part := range strings.Split(p, "/") {
		if strings.EqualFold(part, ".ppgit") || strings.EqualFold(part, ".git") {
			return true
		}
	}
	result := false
	for _, r := range rules {
		if r.matches(p) {
			result = !r.Negative
		}
	}
	return result
}

func keepIgnoredDir(p string, rules []ignoreRule) bool {
	for _, part := range strings.Split(p, "/") {
		if strings.EqualFold(part, ".ppgit") || strings.EqualFold(part, ".git") {
			return false
		}
	}
	for _, r := range rules {
		if r.Negative && (!r.Slash || r.Pattern == p || strings.HasPrefix(r.Pattern, p+"/")) {
			return true
		}
	}
	return false
}

func filterTree(tree Snapshot, rules []ignoreRule) Snapshot {
	out := make(Snapshot, len(tree))
	for p, e := range tree {
		if !ignored(p, rules) {
			out[p] = e
		}
	}
	return out
}

type workspaceFile struct {
	Rel, Path string
	Info      os.FileInfo
	Signature string
}

func workspaceFiles(p project, rules []ignoreRule) ([]workspaceFile, string, error) {
	if err := guardAbsolute(p.Path); err != nil {
		return nil, "", err
	}
	files := make([]workspaceFile, 0)
	hash := sha256.New()
	err := filepath.WalkDir(p.Path, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == p.Path {
			return nil
		}
		rel, err := filepath.Rel(p.Path, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if ignored(rel, rules) {
			if d.IsDir() {
				if !keepIgnoredDir(rel, rules) {
					return filepath.SkipDir
				}
			} else {
				return nil
			}
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if isLink(info) {
			return fmt.Errorf("workspace link is not allowed: %s", rel)
		}
		if d.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported workspace file: %s", rel)
		}
		if err = ValidatePath(rel); err != nil {
			return err
		}
		sig, err := metadataSignature(path, info)
		if err != nil {
			return err
		}
		files = append(files, workspaceFile{rel, path, info, sig})
		fmt.Fprintf(hash, "%s\x00%s\x00", rel, sig)
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	return files, hex.EncodeToString(hash.Sum(nil)), nil
}

func looksText(raw []byte) bool {
	if bytes.IndexByte(raw, 0) >= 0 {
		return false
	}
	bad := 0
	for _, b := range raw {
		if b < 32 && b != 7 && b != 8 && b != 9 && b != 10 && b != 12 && b != 13 && b != 27 {
			bad++
		}
	}
	return len(raw) == 0 || float64(bad)/float64(len(raw)) <= 0.30
}

type binaryDetector struct{ hasNUL bool }

func (d *binaryDetector) Write(p []byte) (int, error) {
	d.hasNUL = d.hasNUL || bytes.IndexByte(p, 0) >= 0
	return len(p), nil
}

func stableFile(p project, file workspaceFile, persist bool, stage string) (Entry, string, error) {
	for attempt := 0; attempt < 3; attempt++ {
		before, err := os.Lstat(file.Path)
		if err != nil {
			return Entry{}, "", err
		}
		if isLink(before) || !before.Mode().IsRegular() {
			return Entry{}, "", errors.New("file changed into unsupported file type")
		}
		beforeSignature, err := metadataSignature(file.Path, before)
		if err != nil {
			return Entry{}, "", err
		}
		f, err := os.Open(file.Path)
		if err != nil {
			return Entry{}, "", err
		}
		opened, err := f.Stat()
		if err != nil {
			f.Close()
			return Entry{}, "", err
		}
		if !os.SameFile(before, opened) {
			f.Close()
			continue
		}
		hash := sha256.New()
		prefix := make([]byte, 8192)
		n, readErr := io.ReadFull(f, prefix)
		if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
			f.Close()
			return Entry{}, "", readErr
		}
		prefix = prefix[:n]
		detector := &binaryDetector{}
		var temp *os.File
		tempName := ""
		var writer io.Writer = io.MultiWriter(hash, detector)
		if persist {
			tempRoot := stage
			if tempRoot == "" {
				tempRoot, err = metaPath(p, "staging")
				if err != nil {
					f.Close()
					return Entry{}, "", err
				}
			}
			temp, err = os.CreateTemp(tempRoot, "blob-*")
			if err != nil {
				f.Close()
				return Entry{}, "", err
			}
			tempName = temp.Name()
			writer = io.MultiWriter(hash, temp, detector)
		}
		_, err = writer.Write(prefix)
		var rest int64
		if err == nil {
			buffer := copyBuffers.Get().(*[]byte)
			rest, err = io.CopyBuffer(writer, f, *buffer)
			copyBuffers.Put(buffer)
		}
		afterHandle, statErr := f.Stat()
		closeErr := f.Close()
		after, pathErr := os.Lstat(file.Path)
		if temp != nil {
			if err == nil {
				err = temp.Sync()
			}
			e := temp.Close()
			if err == nil {
				err = e
			}
		}
		if err != nil || statErr != nil || closeErr != nil || pathErr != nil {
			if tempName != "" {
				os.Remove(tempName)
			}
			return Entry{}, "", errors.Join(err, statErr, closeErr, pathErr)
		}
		afterSignature, err := metadataSignature(file.Path, after)
		if err != nil {
			if tempName != "" {
				os.Remove(tempName)
			}
			return Entry{}, "", err
		}
		if beforeSignature != afterSignature || signature(before) != signature(after) || signature(before) != signature(afterHandle) || !os.SameFile(before, after) || int64(n)+rest != before.Size() {
			if tempName != "" {
				os.Remove(tempName)
			}
			continue
		}
		id := hex.EncodeToString(hash.Sum(nil))
		isText := !detector.hasNUL && looksText(prefix)
		mimeType := mime.TypeByExtension(filepath.Ext(file.Rel))
		if mimeType == "" {
			if isText {
				mimeType = "text/plain"
			} else {
				mimeType = "application/octet-stream"
			}
		}
		e := Entry{id, before.Size(), isText, mimeType, before.ModTime().UnixNano(), beforeSignature, uint32(before.Mode().Perm())}
		newObject := ""
		if persist {
			destination, err := objectPath(p, id)
			if err != nil {
				os.Remove(tempName)
				return Entry{}, "", err
			}
			if info, e := os.Stat(destination); e == nil {
				if !info.Mode().IsRegular() || info.Size() != before.Size() {
					os.Remove(tempName)
					return Entry{}, "", errors.New("existing object is corrupt")
				}
				os.Remove(tempName)
			} else if !os.IsNotExist(e) {
				os.Remove(tempName)
				return Entry{}, "", e
			} else {
				if stage != "" {
					destination = filepath.Join(stage, id)
				} else {
					if err = os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
						os.Remove(tempName)
						return Entry{}, "", err
					}
				}
				if err = os.Rename(tempName, destination); err != nil {
					os.Remove(tempName)
					return Entry{}, "", err
				}
				newObject = id
			}
		}
		return e, newObject, nil
	}
	return Entry{}, "", fmt.Errorf("file changed repeatedly during scan: %s", file.Rel)
}

func (c *Core) scan(p project, reuse Snapshot, rules []ignoreRule, persist bool, stage string, progress func(int, int)) (Snapshot, string, []string, error) {
	files, token, err := workspaceFiles(p, rules)
	if err != nil {
		return nil, "", nil, err
	}
	out := make(Snapshot, len(files))
	created := []string{}
	if progress != nil {
		progress(0, len(files))
	}
	for i, file := range files {
		entry, ok := reuse[file.Rel]
		if !ok || entry.Signature != file.Signature {
			var id string
			entry, id, err = stableFile(p, file, persist, stage)
			if err != nil {
				return nil, "", created, err
			}
			if id != "" {
				created = append(created, id)
			}
		}
		out[file.Rel] = entry
		if progress != nil && (i%64 == 0 || i == len(files)-1) {
			progress(i+1, len(files))
		}
	}
	return out, token, created, nil
}

func (c *Core) GetStatus(id string, quick bool) (map[string]any, error) {
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
	v, _, err := findVersion(s, s.Head)
	if err != nil {
		return nil, err
	}
	tree, err := c.loadTree(p, v.Tree)
	if err != nil {
		return nil, err
	}
	rules, err := loadIgnores(p)
	if err != nil {
		return nil, err
	}
	base := filterTree(tree, rules)
	current, _, _, err := c.scan(p, base, rules, false, "", nil)
	if err != nil {
		return nil, err
	}
	files := DiffSnapshots(base, current)
	count := len(files)
	if quick {
		files = []map[string]any{}
	}
	return map[string]any{"count": count, "files": files}, nil
}

func publicJob(job *scanJob, include bool) map[string]any {
	// @Copyright Electric Reverse

	var e any
	if job.Error != "" {
		e = job.Error
	}
	out := map[string]any{"scan_id": job.ID, "state": job.State, "processed": job.Processed, "total": job.Total, "count": len(job.Files), "error": e}
	if include {
		files := job.Files
		if files == nil {
			files = []map[string]any{}
		}
		out["files"] = files
	}
	return out
}

func (c *Core) pruneJobs() {
	c.jobsMu.Lock()
	defer c.jobsMu.Unlock()
	for id, j := range c.jobs {
		if j.State != "running" && time.Since(j.Access) > 10*time.Minute {
			if j.Stage != "" {
				os.RemoveAll(j.Stage)
			}
			delete(c.jobs, id)
		}
	}
}

func (c *Core) StartStatusScan(id string) (map[string]any, error) {
	p, l, err := c.require(id)
	if err != nil {
		return nil, err
	}
	c.pruneJobs()
	c.jobsMu.Lock()
	if c.closing {
		c.jobsMu.Unlock()
		return nil, errors.New("PPGit is shutting down; new scans are disabled")
	}
	for _, j := range c.jobs {
		if j.ProjectID == id && j.State == "running" {
			out := publicJob(j, true)
			c.jobsMu.Unlock()
			return out, nil
		}
	}
	for scan, j := range c.jobs {
		if j.ProjectID == id && j.State != "running" {
			if j.Stage != "" {
				os.RemoveAll(j.Stage)
			}
			delete(c.jobs, scan)
		}
	}
	for len(c.jobs) >= 8 {
		var oldest string
		var age time.Time
		for key, j := range c.jobs {
			if j.State != "running" && (oldest == "" || j.Access.Before(age)) {
				oldest, age = key, j.Access
			}
		}
		if oldest == "" {
			c.jobsMu.Unlock()
			return nil, errors.New("too many active scans")
		}
		if c.jobs[oldest].Stage != "" {
			os.RemoveAll(c.jobs[oldest].Stage)
		}
		delete(c.jobs, oldest)
	}
	j := &scanJob{ID: newID(), ProjectID: id, State: "running", Access: time.Now(), Files: []map[string]any{}}
	c.jobs[j.ID] = j
	out := publicJob(j, true)
	c.scanWorkers.Add(1)
	c.jobsMu.Unlock()
	go func() {
		defer c.scanWorkers.Done()
		l.Lock()
		defer l.Unlock()
		var scanErr error
		var snap Snapshot
		var stagedObjects []string
		var token, base, stage string
		defer func() {
			c.jobsMu.Lock()
			defer c.jobsMu.Unlock()
			j.Access = time.Now()
			if scanErr != nil {
				j.State = "failed"
				j.Error = scanErr.Error()
				if stage != "" {
					os.RemoveAll(stage)
				}
			} else {
				j.State = "completed"
				j.Snapshot = snap
				j.Objects = stagedObjects
				j.Token = token
				j.Base = base
				j.Stage = stage
				if len(snap) > 200000 {
					j.Snapshot = nil
					if stage != "" {
						os.RemoveAll(stage)
					}
					j.Stage = ""
				}
			}
		}()
		var s state
		s, scanErr = c.loadState(p)
		if scanErr != nil {
			return
		}
		var v version
		v, _, scanErr = findVersion(s, s.Head)
		if scanErr != nil {
			return
		}
		var tree Snapshot
		tree, scanErr = c.loadTree(p, v.Tree)
		if scanErr != nil {
			return
		}
		var rules []ignoreRule
		rules, scanErr = loadIgnores(p)
		if scanErr != nil {
			return
		}
		base = s.Head
		var stageRoot string
		stageRoot, scanErr = metaPath(p, "staging")
		if scanErr != nil {
			return
		}
		stage, scanErr = os.MkdirTemp(stageRoot, "scan-")
		if scanErr != nil {
			return
		}
		c.jobsMu.Lock()
		j.Stage = stage
		c.jobsMu.Unlock()
		tree = filterTree(tree, rules)
		snap, token, stagedObjects, scanErr = c.scan(p, tree, rules, true, stage, func(processed, total int) {
			c.jobsMu.Lock()
			j.Processed = processed
			j.Total = total
			c.jobsMu.Unlock()
		})
		if scanErr == nil {
			c.jobsMu.Lock()
			j.Files = DiffSnapshots(tree, snap)
			c.jobsMu.Unlock()
		}
	}()
	return out, nil
}

func (c *Core) GetStatusScan(id, scan string, include bool) (map[string]any, error) {
	if _, _, err := c.require(id); err != nil {
		return nil, err
	}
	c.pruneJobs()
	c.jobsMu.Lock()
	defer c.jobsMu.Unlock()
	j := c.jobs[scan]
	if j == nil || j.ProjectID != id {
		return nil, errors.New("scan does not exist")
	}
	j.Access = time.Now()
	return publicJob(j, include), nil
}

func (c *Core) discardProjectScans(id string) {
	c.jobsMu.Lock()
	defer c.jobsMu.Unlock()
	for scan, j := range c.jobs {
		if j.ProjectID == id && j.State != "running" {
			if j.Stage != "" {
				os.RemoveAll(j.Stage)
			}
			delete(c.jobs, scan)
		}
	}
}
