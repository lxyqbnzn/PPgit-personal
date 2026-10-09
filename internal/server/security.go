package server

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

func authority(value string) (string, string, error) {
	parsed, err := url.Parse("//" + value)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || strings.ContainsAny(value, " \\\t\r\n") {
		return "", "", bad("Invalid Host header.")
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if ip, err := netip.ParseAddr(host); err == nil {
		host = ip.String()
	}
	port := parsed.Port()
	if port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", "", bad("Invalid Host port.")
		}
		port = strconv.Itoa(n)
	}
	return host, port, nil
}

func allowedHosts(host string, trusted []string) (map[string]bool, error) {
	result := map[string]bool{"localhost": true, "127.0.0.1": true, "::1": true}
	if host != "" && host != "0.0.0.0" && host != "::" && host != "[::]" {
		trusted = append(append([]string{}, trusted...), host)
	}
	for _, value := range trusted {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if strings.Count(value, ":") > 1 && !strings.HasPrefix(value, "[") {
			value = "[" + value + "]"
		}
		name, _, err := authority(value)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted host %q", value)
		}
		result[name] = true
	}
	return result, nil
}

func origin(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return "", bad("Invalid origin.")
	}
	host, port, err := authority(parsed.Host)
	if err != nil {
		return "", err
	}
	if port == "" {
		if parsed.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return parsed.Scheme + "|" + host + "|" + port, nil
}

func secureEqual(a, b string) bool {
	return a != "" && b != "" && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func (s *Server) authorize(r *http.Request) error {
	host, _, err := authority(r.Host)
	if err != nil {
		return err
	}
	if !s.allowed[host] {
		return bad("Host is not allowed.")
	}
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		return nil
	}
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return &httpError{403, "Cross-site API requests are not allowed."}
	}
	if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
		return nil
	}
	if values, present := r.Header["Origin"]; present {
		if len(values) != 1 {
			return &httpError{403, "Request origin is not allowed."}
		}
		actual, err := origin(values[0])
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		expected, _ := origin(scheme + "://" + r.Host)
		if err != nil || actual != expected {
			return &httpError{403, "Request origin is not allowed."}
		}
	}
	if !secureEqual(r.Header.Get("X-PPGit-Token"), s.token) {
		return &httpError{403, "Invalid request token. Refresh the PPGit page and retry."}
	}
	return nil
}
