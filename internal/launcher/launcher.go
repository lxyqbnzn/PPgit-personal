package launcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type Instance struct {
	URL           string `json:"url"`
	ID            string `json:"instance_id"`
	Token         string `json:"token"`
	ShutdownToken string `json:"shutdown_token"`
	PID           int    `json:"pid"`
}

func Root(executable string) string {
	dir := filepath.Dir(executable)
	if filepath.Base(dir) == "bin" {
		return filepath.Dir(dir)
	}
	return dir
}

func DataDir(executable string) string {
	// @Copyright Electric Reverse

	if value := os.Getenv("PPGIT_DATA_DIR"); value != "" {
		return value
	}
	return filepath.Join(Root(executable), "data")
}

func client(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func Read(path string) (Instance, error) {
	// @Copyright Electric Reverse

	var info Instance
	f, err := os.Open(path)
	if err != nil {
		return info, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 16*1024+1))
	if err != nil {
		return info, err
	}
	if len(raw) > 16*1024 {
		return info, errors.New("invalid runtime metadata")
	}
	if err = json.Unmarshal(raw, &info); err != nil {
		return info, err
	}
	parsed, err := url.Parse(info.URL)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return info, errors.New("invalid runtime address")
	}
	ip, ipErr := netip.ParseAddr(parsed.Hostname())
	if parsed.Hostname() != "localhost" && (ipErr != nil || !ip.IsLoopback()) {
		return info, errors.New("refusing a non-loopback service address")
	}
	c := client(2 * time.Second)
	defer c.CloseIdleConnections()
	response, err := c.Get(info.URL + "/api/health")
	if err != nil {
		return info, err
	}
	defer response.Body.Close()
	var health struct {
		ID             string `json:"instance_id"`
		Implementation string `json:"implementation"`
		Service        string `json:"service"`
	}
	err = json.NewDecoder(io.LimitReader(response.Body, 16384)).Decode(&health)
	if err != nil || response.StatusCode != 200 || info.ID == "" || health.ID != info.ID || health.Implementation != "go" || health.Service != "ppgit" {
		return info, errors.New("running service identity does not match; no process was stopped")
	}
	return info, nil
}

func Stop(path string) error {
	info, err := Read(path)
	if err != nil {
		return fmt.Errorf("no matching running PPGit Go instance: %w", err)
	}
	return requestStop(info)
}

func requestStop(info Instance) error {
	request, err := http.NewRequest("POST", info.URL+"/api/shutdown", nil)
	if err != nil {
		return err
	}
	request.Header.Set("X-PPGit-Token", info.Token)
	request.Header.Set("X-PPGit-Shutdown-Token", info.ShutdownToken)
	c := client(5 * time.Second)
	defer c.CloseIdleConnections()
	response, err := c.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return fmt.Errorf("stop refused: HTTP %d", response.StatusCode)
	}
	return nil
}
