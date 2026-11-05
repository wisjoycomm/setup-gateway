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
		return errors.New("-auth phải là gateway, oauth, api-key hoặc login")
	}
	if strings.ContainsAny(p.Credential, "\r\n\t ") {
		return errors.New("key/token phải liền một dòng, không có khoảng trắng")
	}
	if strings.HasPrefix(p.Credential, "sk-ant-sid") || strings.HasPrefix(p.Credential, "sessionKey=") {
		return errors.New("cookie sessionKey (sk-ant-sid...) của claude.ai không dùng được; hãy dùng token sk-ant-oat, API key hoặc đăng nhập web")
	}
	if p.Auth == "login" {
		if p.Credential != "" || p.BaseURL != "" {
			return errors.New("đăng nhập web không cần key hay domain")
		}
		p.Header = ""
		p.Models = nil
		return nil
	}
	if p.Credential == "" {
		return errors.New("chưa nhập key/token")
	}
	if p.Auth == "oauth" {
		if !strings.HasPrefix(p.Credential, "sk-ant-oat") || len(p.Credential) <= len("sk-ant-oat") {
			return errors.New("token OAuth phải là chuỗi đầy đủ bắt đầu bằng sk-ant-oat...")
		}
		if p.BaseURL != "" {
			return errors.New("token OAuth nối thẳng tới Claude, không đi qua domain; dùng domain thì chọn Gateway với key do gateway cấp")
		}
		p.Header = ""
		p.Models = nil
		return nil
	}
	if strings.HasPrefix(p.Credential, "sk-ant-oat") {
		return errors.New("đây là token OAuth (sk-ant-oat); hãy chọn mục Token OAuth (-auth oauth), không dán vào Gateway/API key")
	}
	if p.Auth == "api-key" {
		if p.BaseURL != "" && p.BaseURL != "https://api.anthropic.com/v1" {
			return errors.New("có domain riêng thì chọn Gateway; API key luôn dùng máy chủ Anthropic chính thức")
		}
		p.BaseURL = "https://api.anthropic.com/v1"
		p.Header = "x-api-key"
		return nil
	}
	if p.Header == "" {
		p.Header = "bearer"
	}
	if p.Header != "bearer" && p.Header != "x-api-key" {
		return errors.New("-auth-header phải là bearer hoặc x-api-key")
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
		return "", errors.New("chưa nhập domain gateway")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return "", errors.New("domain gateway không hợp lệ")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("domain gateway không được chứa user:pass, ?tham số hay #")
	}
	loopback := strings.EqualFold(u.Hostname(), "localhost") || net.ParseIP(u.Hostname()).IsLoopback()
	if u.Scheme != "https" && !(u.Scheme == "http" && loopback) {
		return "", errors.New("gateway phải dùng https:// (http:// chỉ cho localhost)")
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
		return nil, errors.New("địa chỉ /models không hợp lệ")
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
		return nil, errors.New("không kết nối được gateway; kiểm tra domain, mạng, chứng chỉ TLS (hoặc proxy)")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		// Some gateways echo credentials in errors. Report status, never the body.
		return nil, fmt.Errorf("gateway trả về HTTP %d khi kiểm tra key; 401/403 = sai key hoặc sai loại key (thử -auth-header x-api-key). Gateway không có /models thì dùng -skip-check -models ID1,ID2", res.StatusCode)
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&payload); err != nil {
		return nil, errors.New("gateway không trả về danh sách model đúng dạng")
	}
	var ids []string
	for _, item := range payload.Data {
		if item.ID != "" && !contains(ids, item.ID) {
			ids = append(ids, item.ID)
		}
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return nil, errors.New("gateway trả về danh sách model rỗng")
	}
	return ids, nil
}

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func validName(name string) bool { return namePattern.MatchString(name) }

func profilePath(home, name string) (string, error) {
	if !validName(name) {
		return "", errors.New("tên profile chỉ gồm chữ, số, _ hoặc - (vd adnx, work)")
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
		return p, fmt.Errorf("không có profile %q; xem danh sách: setup-gateway -list-profiles", name)
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return p, fmt.Errorf("file profile %q bị hỏng", name)
	}
	err = p.validate()
	return p, err
}

func listProfiles(home string, out io.Writer) error {
	entries, err := os.ReadDir(filepath.Join(home, ".setup-gateway", "profiles"))
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(out, "Chưa lưu profile nào. Khi kết nối Claude, đặt tên ở câu hỏi cuối (hoặc thêm -profile TÊN).")
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
			fmt.Fprintf(out, "  %s: profile bị lỗi\n", name)
			continue
		}
		fmt.Fprintf(out, "  %s: %s %s\n", name, p.Auth, p.BaseURL)
	}
	return nil
}
