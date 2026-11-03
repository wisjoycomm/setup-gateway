package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func TestURLNormalization(t *testing.T) {
	for raw, want := range map[string]string{
		"ai.example.com":                           "https://ai.example.com/v1",
		"https://ai.example.com/V1/models":         "https://ai.example.com/v1",
		"https://ai.example.com/proxy/v1/messages": "https://ai.example.com/proxy/v1",
		"https://ai.example.com/proxy/":            "https://ai.example.com/proxy/v1",
		"http://127.0.0.1:9000":                    "http://127.0.0.1:9000/v1",
		"http://[::1]:9000":                        "http://[::1]:9000/v1",
	} {
		got, err := gatewayURL(raw)
		if err != nil || got != want {
			t.Fatalf("%q -> %q, %v", raw, got, err)
		}
	}
	for _, raw := range []string{"", "http://remote.example", "ftp://localhost", "https://user:secret@example.com", "https://example.com?key=secret", "https://example.com#secret", "https://"} {
		if _, err := gatewayURL(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}

func TestCredentialModes(t *testing.T) {
	cases := []struct {
		p     connectionProfile
		valid bool
	}{
		{connectionProfile{Auth: "oauth", Credential: "sk-ant-oat-test"}, true},
		{connectionProfile{Auth: "session-key", Credential: "sk-ant-oat-test"}, true},
		{connectionProfile{Auth: "oauth", Credential: "sk-ant-sid-test"}, false},
		{connectionProfile{Auth: "oauth", Credential: "sk-ant-oat-test", BaseURL: "https://wrong.example"}, false},
		{connectionProfile{Auth: "gateway", Credential: "sk-ant-oat-test", BaseURL: "https://example.com"}, false},
		{connectionProfile{Auth: "api-key", Credential: "sk-ant-oat-test"}, false},
		{connectionProfile{Auth: "gateway", Credential: "test", BaseURL: "https://example.com", Header: "x-api-key"}, true},
		{connectionProfile{Auth: "gateway", Credential: "test\nsecret", BaseURL: "https://example.com"}, false},
		{connectionProfile{Auth: "api-key", Credential: "test"}, true},
		{connectionProfile{Auth: "api-key", Credential: "test", BaseURL: "https://example.com"}, false},
		{connectionProfile{Auth: "login"}, true},
		{connectionProfile{Auth: "login", Credential: "test"}, false},
	}
	for _, c := range cases {
		if err := c.p.validate(); (err == nil) != c.valid {
			t.Fatalf("mode %s: valid=%t err=%v", c.p.Auth, c.valid, err)
		}
	}
}

func TestModelDiscoveryHeadersAndErrors(t *testing.T) {
	for _, header := range []string{"bearer", "x-api-key"} {
		t.Run(header, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/models" || r.Method != "GET" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				if header == "bearer" && (r.Header.Get("Authorization") != "Bearer secret-test" || r.Header.Get("x-api-key") != "") {
					t.Error("wrong bearer header")
				}
				if header == "x-api-key" && (r.Header.Get("x-api-key") != "secret-test" || r.Header.Get("Authorization") != "") {
					t.Error("wrong API-key header")
				}
				if r.Header.Get("anthropic-version") == "" {
					t.Error("missing version")
				}
				fmt.Fprint(w, `{"data":[{"id":"claude-sonnet"},{"id":"claude-opus"},{"id":"claude-sonnet"}]}`)
			}))
			defer server.Close()
			ids, err := discoverModels(connectionProfile{BaseURL: server.URL + "/v1", Credential: "secret-test", Header: header})
			if err != nil || len(ids) != 2 || ids[0] != "claude-opus" {
				t.Fatalf("%v %v", ids, err)
			}
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401); fmt.Fprint(w, "secret-test") }))
	defer server.Close()
	_, err := discoverModels(connectionProfile{BaseURL: server.URL + "/v1", Credential: "secret-test"})
	if err == nil || strings.Contains(err.Error(), "secret-test") || !strings.Contains(err.Error(), "401") {
		t.Fatalf("unsafe error: %v", err)
	}
}

func TestDiscoveryDoesNotFollowRedirects(t *testing.T) {
	var calls atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, http.StatusFound) }))
	defer source.Close()
	_, err := discoverModels(connectionProfile{BaseURL: source.URL + "/v1", Credential: "secret-test", Header: "x-api-key"})
	if err == nil || calls.Load() != 0 {
		t.Fatal("credential-bearing redirect was followed")
	}
}

func TestCLIDryRunNoNetworkAndFailedCheckNoWrites(t *testing.T) {
	home := testHome(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(401) }))
	defer server.Close()
	args := []string{"-auth", "gateway", "-base-url", server.URL, "-key", "fake-key", "-only", "claude"}
	var out bytes.Buffer
	if err := runCLIWithHome(append(args, "-dry-run"), strings.NewReader(""), &out, home); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 {
		t.Fatal("dry run contacted gateway")
	}
	if err := runCLIWithHome(args, strings.NewReader(""), &out, home); err == nil {
		t.Fatal("expected failed check")
	}
	if calls.Load() != 1 {
		t.Fatal("unexpected check count")
	}
	if _, err := os.Stat(claudeSettingsPath(home)); !os.IsNotExist(err) {
		t.Fatal("failed validation wrote settings")
	}
	if err := runCLIWithHome([]string{"-auth", "oauth", "-key", "sk-ant-oat-test", "-only", "claude,unknown"}, strings.NewReader(""), &out, home); err == nil {
		t.Fatal("unknown target accepted")
	}
	if _, err := os.Stat(claudeSettingsPath(home)); !os.IsNotExist(err) {
		t.Fatal("invalid target partially applied")
	}
}
