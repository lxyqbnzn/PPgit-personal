package core

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const MaxPreviewSize int64 = 10 * 1024 * 1024
const formatVersion = "ppgit-go-1"

var ErrTooLarge = errors.New("file exceeds preview limit")
var ErrVersionNotFound = errors.New("version does not exist")
var storageID = regexp.MustCompile(`^[a-f0-9]{64}$`)
var versionID = regexp.MustCompile(`^[a-f0-9]{32}$`)
var reservedName = regexp.MustCompile(`^(COM|LPT)[1-9]$`)

type Entry struct {
	ObjectID  string `json:"object_id"`
	Size      int64  `json:"size"`
	IsText    bool   `json:"is_text"`
	MIMEType  string `json:"mime_type"`
	ModTime   int64  `json:"mtime_ns"`
	Signature string `json:"signature"`
	Mode      uint32 `json:"mode"`
}
type Snapshot map[string]Entry
type version struct {
	ID             string `json:"version_id"`
	Parent         string `json:"parent_version_id,omitempty"`
	Message        string `json:"message"`
	Created        string `json:"created_at"`
	Tree           string `json:"tree_id"`
	Changed        int    `json:"changed_files_count"`
	ReadOnlyReason string `json:"read_only_reason,omitempty"`
}
type state struct {
	Format   string    `json:"format"`
	Head     string    `json:"head_version_id"`
	Versions []version `json:"versions"`
}
type project struct {
	ID   string `json:"project_id"`
	Name string `json:"name"`
	Path string `json:"path"`
}
type config struct {
	Projects []project `json:"projects"`
}
type cacheTree struct {
	Signature string
	Snapshot  Snapshot
	Access    time.Time
}
type cacheState struct {
	Signature string
	State     state
	Access    time.Time
}
type scanJob struct {
	ID, ProjectID, State, Error, Base, Token, Stage string
	Processed, Total                                int
	Files                                           []map[string]any
	Snapshot                                        Snapshot
	Objects                                         []string
	Access                                          time.Time
}
type Core struct {
	dataDir      string
	mu           sync.Mutex
	projects     []project
	locks        map[string]*sync.Mutex
	jobsMu       sync.Mutex
	jobs         map[string]*scanJob
	scanWorkers  sync.WaitGroup
	closing      bool
	cacheMu      sync.Mutex
	trees        map[string]cacheTree
	states       map[string]cacheState
	checkoutHook func(string) error
}

func New(dataDir string) (*Core, error) {
	abs, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, err
	}
	if err = guardAbsolute(abs); err != nil {
		return nil, err
	}
	if err = os.MkdirAll(abs, 0700); err != nil {
		return nil, err
	}
	c := &Core{dataDir: abs, locks: map[string]*sync.Mutex{}, jobs: map[string]*scanJob{}, trees: map[string]cacheTree{}, states: map[string]cacheState{}}
	file := filepath.Join(abs, "config.json")
	if err = guardAbsolute(file); err != nil {
		return nil, err
	}
	var cfg config
	if err = readJSON(file, &cfg); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read project config: %w", err)
		}
		cfg.Projects = []project{}
		if err = writeJSON(file, cfg); err != nil {
			return nil, err
		}
	}
	c.projects = cfg.Projects
	return c, nil
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

func (c *Core) require(id string) (project, *sync.Mutex, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range c.projects {
		if p.ID == id {
			key := p.Path
			if runtime.GOOS == "windows" {
				key = strings.ToLower(key)
			}
			l := c.locks[key]
			if l == nil {
				l = &sync.Mutex{}
				c.locks[key] = l
			}
			return p, l, nil
		}
	}
	return project{}, nil, errors.New("project does not exist")
}

func ValidatePath(p string) error {
	// @Copyright Electric Reverse

	if p == "" || strings.ContainsAny(p, "\\:\x00<>\"|?*") || strings.HasPrefix(p, "/") || filepath.IsAbs(p) {
		return fmt.Errorf("unsafe snapshot path: %q", p)
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." || strings.EqualFold(part, ".ppgit") || strings.EqualFold(part, ".git") || strings.TrimRight(part, " .") != part {
			return fmt.Errorf("unsafe snapshot path: %q", p)
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || reservedName.MatchString(base) {
			return fmt.Errorf("reserved path: %q", p)
		}
	}
	return nil
}

func guardAbsolute(p string) error {
	// @Copyright Electric Reverse

	abs, err := filepath.Abs(p)
	if err != nil {
		return err
	}
	vol := filepath.VolumeName(abs)
	cur := vol + string(os.PathSeparator)
	for _, part := range strings.Split(strings.TrimPrefix(abs, cur), string(os.PathSeparator)) {
		if part == "" {
			continue
		}
		cur = filepath.Join(cur, part)
		info, e := os.Lstat(cur)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return e
		}
		if isLink(info) {
			return fmt.Errorf("symbolic links and reparse points are not allowed: %s", cur)
		}
	}
	return nil
}

func safePath(root, rel string) (string, error) {
	if err := ValidatePath(rel); err != nil {
		return "", err
	}
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := guardAbsolute(p); err != nil {
		return "", err
	}
	return p, nil
}

func metaPath(p project, parts ...string) (string, error) {
	// @Copyright Electric Reverse

	x := filepath.Join(append([]string{p.Path, ".ppgit"}, parts...)...)
	if err := guardAbsolute(x); err != nil {
		return "", err
	}
	return x, nil
}

func validateSnapshot(s Snapshot) error {
	if s == nil {
		return errors.New("invalid null snapshot")
	}
	seen := make(map[string]bool, len(s))
	for p, e := range s {
		if err := ValidatePath(p); err != nil {
			return err
		}
		if !storageID.MatchString(e.ObjectID) || e.Size < 0 {
			return fmt.Errorf("invalid snapshot entry: %s", p)
		}
		key := strings.ToLower(p)
		if seen[key] {
			return fmt.Errorf("conflicting snapshot paths: %s", p)
		}
		seen[key] = true
	}
	for p := range seen {
		for parent := filepath.ToSlash(filepath.Dir(p)); parent != "."; parent = filepath.ToSlash(filepath.Dir(parent)) {
			if seen[parent] {
				return fmt.Errorf("file/directory conflict: %s", p)
			}
		}
	}
	return nil
}

func serializeVersion(v version, seq int) map[string]any {
	var parent any
	if v.Parent != "" {
		parent = v.Parent
	}
	return map[string]any{"version_id": v.ID, "short_version_id": v.ID[:8], "message": v.Message, "timestamp": v.Created, "changed_files": v.Changed, "parent_version_id": parent, "sequence": seq, "read_only_reason": v.ReadOnlyReason, "tree_id": v.Tree}
}

func findVersion(s state, id string) (version, int, error) {
	for i, v := range s.Versions {
		if v.ID == id {
			return v, i, nil
		}
	}
	return version{}, 0, ErrVersionNotFound
}

func DiffSnapshots(left, right Snapshot) []map[string]any {
	out := make([]map[string]any, 0)
	for p, l := range left {
		r, ok := right[p]
		if !ok {
			out = append(out, map[string]any{"path": p, "status": "deleted", "size": l.Size})
		} else if l.ObjectID != r.ObjectID {
			out = append(out, map[string]any{"path": p, "status": "modified", "size": r.Size})
		}
	}
	for p, r := range right {
		if _, ok := left[p]; !ok {
			out = append(out, map[string]any{"path": p, "status": "added", "size": r.Size})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["path"].(string) < out[j]["path"].(string) })
	return out
}
