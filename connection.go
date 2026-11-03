package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type connectionProfile struct {
	Auth       string   `json:"auth"`
	BaseURL    string   `json:"base_url,omitempty"`
	Credential string   `json:"credential,omitempty"`
	Header     string   `json:"auth_header,omitempty"`
	Models     []string `json:"models,omitempty"`
}

func canonicalAuth(auth string) string {
	switch strings.ToLower(strings.TrimSpace(auth)) {
	case "session-key", "session", "oauth":
		return "oauth"
	case "apikey", "api-key":
		return "api-key"
	case "browser", "login":
		return "login"
	case "gateway":
		return "gateway"
	default:
		return auth
	}
}

func (p *connectionProfile) validate() error {
	p.Auth = canonicalAuth(p.Auth)
	if !contains([]string{"gateway", "oauth", "api-key", "login"}, p.Auth) {
		return errors.New("auth must be gateway, oauth, api-key, or login")
	}
	if strings.ContainsAny(p.Credential, "\r\n\t ") {
		return errors.New("credential must be a single token without whitespace")
	}
	if strings.HasPrefix(p.Credential, "sk-ant-sid") || strings.HasPrefix(p.Credential, "sessionKey=") {
		return errors.New("a claude.ai sessionKey cookie cannot authenticate Claude Code; use a sk-ant-oat OAuth token, API key, or browser login")
	}
	if p.Auth == "login" {
		if p.Credential != "" || p.BaseURL != "" {
			return errors.New("normal login does not accept a key or custom domain")
		}
		p.Header = ""
		p.Models = nil
		return nil
	}
	if p.Credential == "" {
		return errors.New("a credential is required")
	}
	if p.Auth == "oauth" {
		if !strings.HasPrefix(p.Credential, "sk-ant-oat") || len(p.Credential) <= len("sk-ant-oat") {
			return errors.New("OAuth/session mode requires the complete sk-ant-oat... token")
		}
		if p.BaseURL != "" {
			return errors.New("OAuth mode connects directly to Claude; use a gateway-issued key for a custom domain")
		}
		p.Header = ""
		p.Models = nil
		return nil
	}
	if strings.HasPrefix(p.Credential, "sk-ant-oat") {
		return errors.New("this is an OAuth token; choose -auth oauth instead of sending it as an API or gateway key")
	}
	if p.Auth == "api-key" {
		if p.BaseURL != "" && p.BaseURL != "https://api.anthropic.com/v1" {
			return errors.New("use -auth gateway for a custom domain; api-key mode uses the official Anthropic API")
		}
		p.BaseURL = "https://api.anthropic.com/v1"
		p.Header = "x-api-key"
		return nil
	}
	if p.Header == "" {
		p.Header = "bearer"
	}
	if p.Header != "bearer" && p.Header != "x-api-key" {
		return errors.New("auth-header must be bearer or x-api-key")
	}
	base, err := gatewayURL(p.BaseURL)
	if err != nil {
		return err
	}
	p.BaseURL = base
	return nil
}

func gatewayURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("a gateway domain is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return "", errors.New("invalid gateway URL")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("gateway URL must not include credentials, query parameters or a fragment")
	}
	loopback := strings.EqualFold(u.Hostname(), "localhost") || net.ParseIP(u.Hostname()).IsLoopback()
	if u.Scheme != "https" && !(u.Scheme == "http" && loopback) {
		return "", errors.New("gateway URL must use HTTPS (HTTP is allowed only on localhost)")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, part := range parts {
		if strings.EqualFold(part, "v1") {
			parts = parts[:i]
			break
		}
	}
	u.Path = strings.TrimRight("/"+strings.Join(parts, "/"), "/") + "/v1"
	u.RawPath = ""
	return u.String(), nil
}

func discoverModels(p connectionProfile) ([]string, error) {
	req, err := http.NewRequest(http.MethodGet, p.BaseURL+"/models", nil)
	if err != nil {
		return nil, errors.New("invalid models endpoint")
	}
	if p.Header == "x-api-key" {
		req.Header.Set("x-api-key", p.Credential)
	} else {
		req.Header.Set("Authorization", "Bearer "+p.Credential)
	}
	req.Header.Set("anthropic-version", "2023-06-01")
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		return nil, errors.New("cannot reach the models endpoint; check the domain, network and TLS certificate")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		// Some gateways echo credentials in errors. Report status, never the body.
		return nil, fmt.Errorf("models check returned HTTP %d; check the credential/header, or use -skip-check -models ID1,ID2 if discovery is unsupported", res.StatusCode)
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&payload); err != nil {
		return nil, errors.New("models endpoint did not return a supported JSON model list")
	}
	var ids []string
	for _, item := range payload.Data {
		if item.ID != "" && !contains(ids, item.ID) {
			ids = append(ids, item.ID)
		}
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return nil, errors.New("models endpoint returned no model IDs")
	}
	return ids, nil
}

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func validName(name string) bool { return namePattern.MatchString(name) }

func profilePath(home, name string) (string, error) {
	if !validName(name) {
		return "", errors.New("invalid profile name; use letters, numbers, underscore or hyphen")
	}
	return filepath.Join(home, ".setup-gateway", "profiles", name+".json"), nil
}

func saveProfile(home, name string, p connectionProfile, dry bool) error {
	path, err := profilePath(home, name)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(path, string(data)+"\n", dry)
}

func loadProfile(home, name string) (connectionProfile, error) {
	var p connectionProfile
	path, err := profilePath(home, name)
	if err != nil {
		return p, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return p, fmt.Errorf("cannot read profile %q", name)
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return p, fmt.Errorf("profile %q is not valid JSON", name)
	}
	err = p.validate()
	return p, err
}

func listProfiles(home string, out io.Writer) error {
	entries, err := os.ReadDir(filepath.Join(home, ".setup-gateway", "profiles"))
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(out, "No saved profiles. Use -profile NAME when setting up Claude.")
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		p, err := loadProfile(home, name)
		if err != nil {
			fmt.Fprintf(out, "%s: invalid profile\n", name)
			continue
		}
		fmt.Fprintf(out, "%s: %s %s\n", name, p.Auth, p.BaseURL)
	}
	return nil
}
