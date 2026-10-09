package server

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	"ppgit-go/internal/core"
	diffpreview "ppgit-go/internal/diff"
	"ppgit-go/web"
)

type Options struct {
	Host          string
	TrustedHosts  []string
	InstanceID    string
	BrowseFolder  func() (string, error)
	Shutdown      func()
	ShutdownToken string
}

type Server struct {
	core     *core.Core
	mux      *http.ServeMux
	token    string
	version  string
	index    []byte
	allowed  map[string]bool
	options  Options
	revision atomic.Uint64
}

type httpError struct {
	code    int
	message string
}

func (e *httpError) Error() string { return e.message }

func bad(message string) error { return &httpError{400, message} }

func New(c *core.Core, options Options) (*Server, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return nil, err
	}
	s := &Server{core: c, mux: http.NewServeMux(), token: hex.EncodeToString(random), options: options}
	var err error
	s.allowed, err = allowedHosts(options.Host, options.TrustedHosts)
	if err != nil {
		return nil, err
	}
	digest := sha256.New()
	err = fs.WalkDir(web.Assets, "static", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		raw, err := web.Assets.ReadFile(name)
		if err != nil {
			return err
		}
		digest.Write([]byte(name))
		digest.Write(raw)
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.version = hex.EncodeToString(digest.Sum(nil))[:16]
	raw, err := web.Assets.ReadFile("templates/index.html")
	if err != nil {
		return nil, err
	}
	tmpl, err := template.New("index").Delims("[[", "]]").Funcs(template.FuncMap{
		"csrf_token":    func() string { return s.token },
		"asset_version": func() string { return s.version },
	}).Parse(string(raw))
	if err != nil {
		return nil, err
	}
	var page bytes.Buffer
	if err := tmpl.Execute(&page, nil); err != nil {
		return nil, err
	}
	s.index = page.Bytes()
	s.routes()
	return s, nil
}

func (s *Server) Token() string {
	// @Copyright Electric Reverse
	return s.token
}

func writeJSON(w http.ResponseWriter, code int, value any) {
	raw, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "JSON encoding failed", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_, _ = w.Write(raw)
}

func fail(w http.ResponseWriter, err error) {
	if errors.Is(err, core.ErrMergeConflict) {
		writeJSON(w, 409, map[string]any{"error": "\u7248\u672c\u6587\u4ef6\u5df2\u53d8\u66f4\uff0c\u8bf7\u5237\u65b0\u5bf9\u6bd4\u540e\u91cd\u8bd5", "code": "merge_conflict"})
		return
	}
	if errors.Is(err, core.ErrVersionReadOnly) {
		writeJSON(w, 403, map[string]any{"error": "\u5df2\u63d0\u4ea4\u6216\u540c\u6b65\u7684\u56e2\u961f\u7248\u672c\u4e0d\u53ef\u6539\u5199", "code": "version_read_only"})
		return
	}
	var confirmation *core.CheckoutConfirmationRequired
	if errors.As(err, &confirmation) {
		writeJSON(w, 409, map[string]any{
			"error": err.Error(), "code": "checkout_confirmation_required",
			"confirmation_token": confirmation.Token, "uncommitted_count": confirmation.UncommittedCount,
		})
		return
	}
	if errors.Is(err, core.ErrCheckoutWorkspaceChanged) {
		writeJSON(w, 409, map[string]any{"error": err.Error(), "code": "checkout_workspace_changed"})
		return
	}
	status := 400
	if errors.Is(err, core.ErrVersionNotFound) {
		status = 404
	}
	if errors.Is(err, fs.ErrPermission) {
		message := "Cannot write to or access this folder. Start PPGit normally using the provided launcher, not a restricted terminal, and check the folder's write permission."
		var pathError *fs.PathError
		folder := ""
		if errors.As(err, &pathError) {
			folder = pathError.Path
		}
		log.Printf("filesystem permission denied: %v", err)
		writeJSON(w, 403, map[string]any{"error": message, "code": "permission_denied", "path": folder})
		return
	}
	var typed *httpError
	if errors.As(err, &typed) {
		status = typed.code
	}
	writeJSON(w, status, map[string]any{"error": err.Error()})
}

func (s *Server) changed(err error) {
	if err == nil {
		s.revision.Add(1)
	}
}

func (s *Server) endpoint(pattern string, code int, f func(*http.Request) (any, error)) {
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		value, err := f(r)
		if err != nil {
			fail(w, err)
			return
		}
		writeJSON(w, code, value)
	})
}

func body(r *http.Request) (map[string]any, error) {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || (media != "application/json" && !strings.HasSuffix(media, "+json")) {
		return nil, bad("Request body must be a JSON object.")
	}
	var obj map[string]any
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&obj); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, &httpError{413, fmt.Sprintf("Request body exceeds %d KiB.", tooLarge.Limit/1024)}
		}
		return nil, bad("Request body must be a JSON object.")
	}
	if obj == nil {
		return nil, bad("Request body must be a JSON object.")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, &httpError{413, fmt.Sprintf("Request body exceeds %d KiB.", tooLarge.Limit/1024)}
		}
		return nil, bad("Request body must contain one JSON object.")
	}
	return obj, nil
}

func field(obj map[string]any, key string, required bool, max int) (string, error) {
	value, found := obj[key]
	if (!found || value == nil) && !required {
		return "", nil
	}
	text, ok := value.(string)
	if !ok || utf8.RuneCountInString(text) > max {
		return "", bad(fmt.Sprintf("%s must be a string of at most %d characters.", key, max))
	}
	text = strings.TrimSpace(text)
	if required && text == "" {
		return "", bad(key + " is required.")
	}
	return text, nil
}

func number(r *http.Request, key string, fallback, max int) (int, error) {
	values, present := r.URL.Query()[key]
	if !present {
		return fallback, nil
	}
	n, err := strconv.Atoi(values[0])
	if err != nil || n < 1 {
		return 0, bad(key + " must be a positive integer.")
	}
	if max > 0 && n > max {
		n = max
	}
	return n, nil
}

func enabled(r *http.Request, key string) bool {
	value := strings.ToLower(r.URL.Query().Get(key))
	return value != "0" && value != "false" && value != "no"
}

func optIn(r *http.Request, key string) bool {
	value := strings.ToLower(r.URL.Query().Get(key))
	return value == "1" || value == "true"
}

func (s *Server) routes() {
	s.endpoint("POST /api/projects/{project}/diff/merge", 200, s.mergeDiff)

	s.mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(s.index)
	})
	s.mux.HandleFunc("GET /static/", s.static)

	s.endpoint("GET /api/health", 200, func(r *http.Request) (any, error) {
		return map[string]any{"service": "ppgit", "implementation": "go", "instance_id": s.options.InstanceID}, nil
	})

	s.endpoint("GET /api/revision", 200, func(r *http.Request) (any, error) {
		return map[string]any{"revision": fmt.Sprintf("%s:%s:%d", s.options.InstanceID, s.token[:16], s.revision.Load())}, nil
	})

	s.endpoint("GET /api/projects", 200, func(r *http.Request) (any, error) {
		projects, err := s.core.ListProjects()
		return map[string]any{"projects": projects}, err
	})

	s.endpoint("POST /api/projects", 201, func(r *http.Request) (any, error) {
		obj, err := body(r)
		if err != nil {
			return nil, err
		}
		folder, err := field(obj, "path", true, 16384)
		if err != nil {
			return nil, err
		}
		if !filepath.IsAbs(folder) {
			return nil, bad("Project path must be absolute.")
		}
		project, err := s.core.AddProject(folder)
		s.changed(err)
		return map[string]any{"project": project}, err
	})

	s.endpoint("POST /api/browse-folder", 200, func(r *http.Request) (any, error) {
		if s.options.BrowseFolder == nil {
			return nil, bad("Folder picker is unavailable. Enter an absolute path.")
		}
		folder, err := s.options.BrowseFolder()
		return map[string]any{"path": folder}, err
	})

	s.endpoint("GET /api/projects/{project}", 200, func(r *http.Request) (any, error) {
		project, err := s.core.GetProject(r.PathValue("project"))
		return map[string]any{"project": project}, err
	})

	s.endpoint("DELETE /api/projects/{project}", 200, func(r *http.Request) (any, error) {
		err := s.core.RemoveProject(r.PathValue("project"))
		s.changed(err)
		return map[string]any{"ok": true}, err
	})

	s.endpoint("GET /api/projects/{project}/versions", 200, func(r *http.Request) (any, error) {
		page, err := number(r, "page", 1, 0)
		if err != nil {
			return nil, err
		}
		perPage, err := number(r, "per_page", 50, 200)
		if err != nil {
			return nil, err
		}
		return s.core.GetHistory(r.PathValue("project"), page, perPage)
	})

	s.endpoint("POST /api/projects/{project}/versions", 201, func(r *http.Request) (any, error) {
		obj, err := body(r)
		if err != nil {
			return nil, err
		}
		message, err := field(obj, "message", true, 4096)
		if err != nil {
			return nil, err
		}
		scan, err := field(obj, "scan_id", false, 64)
		if err != nil {
			return nil, err
		}
		version, err := s.core.CreateVersion(r.PathValue("project"), message, scan)
		s.changed(err)
		if err != nil {
			return nil, err
		}
		project, err := s.core.GetProject(r.PathValue("project"))
		return map[string]any{"version": version, "project": project}, err
	})

	s.endpoint("GET /api/projects/{project}/versions/{version}", 200, func(r *http.Request) (any, error) {
		version, err := s.core.GetVersionDetail(r.PathValue("project"), r.PathValue("version"))
		return map[string]any{"version": version}, err
	})

	deleteVersion := func(r *http.Request) (any, error) {
		cleanup, err := s.core.DeleteVersion(r.PathValue("project"), r.PathValue("version"))
		s.changed(err)
		return map[string]any{"ok": true, "cleanup": cleanup}, err
	}
	s.endpoint("DELETE /api/projects/{project}/versions/{version}", 200, deleteVersion)
	s.endpoint("DELETE /api/projects/{project}/versions/{version}/disk", 200, deleteVersion)

	s.endpoint("GET /api/projects/{project}/status", 200, func(r *http.Request) (any, error) {
		return s.core.GetStatus(r.PathValue("project"), optIn(r, "quick"))
	})

	s.endpoint("POST /api/projects/{project}/status-scans", 202, func(r *http.Request) (any, error) {
		return s.core.StartStatusScan(r.PathValue("project"))
	})

	s.endpoint("GET /api/projects/{project}/status-scans/{scan}", 200, func(r *http.Request) (any, error) {
		return s.core.GetStatusScan(r.PathValue("project"), r.PathValue("scan"), optIn(r, "include_files"))
	})

	s.endpoint("GET /api/projects/{project}/diff", 200, func(r *http.Request) (any, error) {
		q := r.URL.Query()
		a, b := strings.TrimSpace(q.Get("from")), strings.TrimSpace(q.Get("to"))
		if a == "" || b == "" {
			return nil, bad("Both from and to versions are required.")
		}
		return diffpreview.GetDiff(s.core, r.PathValue("project"), a, b, q.Get("path"), enabled(r, "details"), enabled(r, "patch"))
	})

	s.endpoint("GET /api/projects/{project}/tree/{version}", 200, func(r *http.Request) (any, error) {
		tree, err := s.core.GetFileTree(r.PathValue("project"), r.PathValue("version"))
		return map[string]any{"tree": tree}, err
	})

	s.endpoint("GET /api/projects/{project}/file/{version}", 200, func(r *http.Request) (any, error) {
		name := r.URL.Query().Get("path")
		if name == "" {
			return nil, bad("File path is required.")
		}
		return s.core.GetFileContent(r.PathValue("project"), r.PathValue("version"), name)
	})

	s.endpoint("POST /api/projects/{project}/checkout/{version}", 200, func(r *http.Request) (any, error) {
		confirmation := ""
		if r.ContentLength != 0 {
			obj, err := body(r)
			if err != nil {
				return nil, err
			}
			confirmation, err = field(obj, "confirmation_token", false, 64)
			if err != nil {
				return nil, err
			}
		}
		project, err := s.core.CheckoutVersionConfirmed(r.PathValue("project"), r.PathValue("version"), confirmation)
		s.changed(err)
		return map[string]any{"project": project}, err
	})

	s.endpoint("GET /api/status", 200, func(r *http.Request) (any, error) {
		projects, err := s.core.ListProjects()
		if err != nil {
			return nil, err
		}
		for i, p := range projects {
			detail, err := s.core.GetProject(p["project_id"].(string))
			if err != nil {
				p["error"] = err.Error()
			} else {
				projects[i] = detail
			}
		}
		return map[string]any{"projects": projects, "count": len(projects)}, nil
	})

	s.endpoint("POST /api/shutdown", 200, func(r *http.Request) (any, error) {
		if !secureEqual(r.Header.Get("X-PPGit-Shutdown-Token"), s.options.ShutdownToken) || s.options.Shutdown == nil {
			return nil, &httpError{403, "Shutdown requires the local launcher credential."}
		}
		s.options.Shutdown()
		return map[string]any{"ok": true}, nil
	})
}

func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/static/")
	if !fs.ValidPath(name) || strings.HasPrefix(name, ".") {
		fail(w, &httpError{404, "Asset not found."})
		return
	}
	raw, err := web.Assets.ReadFile("static/" + name)
	if err != nil {
		fail(w, &httpError{404, "Asset not found."})
		return
	}
	if r.URL.Query().Get("v") == s.version {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache, max-age=0, must-revalidate")
	}
	w.Header().Del("Pragma")
	w.Header().Del("Expires")
	contentType := mime.TypeByExtension(path.Ext(name))
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	_, _ = w.Write(raw)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// @Copyright Electric Reverse

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'; object-src 'none'; base-uri 'none'")
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("request panic: %v", recovered)
			writeJSON(w, 500, map[string]any{"error": "Internal server error. See the service log."})
		}
	}()
	if err := s.authorize(r); err != nil {
		fail(w, err)
		return
	}
	bodyLimit := int64(64 * 1024)
	if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/projects/") && strings.HasSuffix(r.URL.Path, "/diff/merge") {
		bodyLimit = 256 * 1024
	}
	r.Body = http.MaxBytesReader(w, r.Body, bodyLimit)
	if r.ContentLength > bodyLimit {
		fail(w, &httpError{413, fmt.Sprintf("Request body exceeds %d KiB.", bodyLimit/1024)})
		return
	}
	_, pattern := s.mux.Handler(r)
	if pattern == "" {
		allow := []string{}
		for _, method := range []string{"GET", "HEAD", "POST", "DELETE"} {
			clone := r.Clone(r.Context())
			clone.Method = method
			if _, p := s.mux.Handler(clone); p != "" {
				allow = append(allow, method)
			}
		}
		if len(allow) > 0 {
			w.Header().Set("Allow", strings.Join(allow, ", "))
			fail(w, &httpError{405, "Method not allowed."})
		} else {
			fail(w, &httpError{404, "Not found."})
		}
		return
	}
	s.mux.ServeHTTP(w, r)
}
