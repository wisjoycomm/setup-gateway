package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectOnOffKeepsUserSettings(t *testing.T) {
	home := t.TempDir()
	ca := mitmCAPath(home)
	_ = os.MkdirAll(filepath.Dir(ca), 0o700)
	_ = os.WriteFile(ca, []byte("ca"), 0o600)
	root := filepath.Join(t.TempDir(), "my-proj")
	sub := filepath.Join(root, "src", "deep")
	_ = os.MkdirAll(sub, 0o700)
	_ = os.MkdirAll(filepath.Join(root, ".git"), 0o700)
	path := projectSettingsPath(root)
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	_ = os.WriteFile(path, []byte(`{"permissions":{"allow":["Read"]},"env":{"KEEP":"1"}}`), 0o600)
	var out bytes.Buffer
	if err := projectOn([]string{"-dir", sub}, &out, home); err != nil {
		t.Fatal(err, out.String())
	}
	doc, _ := readJSONObject(path)
	env := doc["env"].(map[string]any)
	if env["HTTPS_PROXY"] != "http://my-proj:x@127.0.0.1:8877" || env["KEEP"] != "1" || doc["permissions"] == nil || env["NODE_EXTRA_CA_CERTS"] == nil {
		t.Fatalf("bad settings %v", doc)
	}
	ignore, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if !strings.Contains(string(ignore), ".claude/settings.local.json") {
		t.Fatal("not gitignored")
	}
	out.Reset()
	if err := projectList(&out, home); err != nil || !strings.Contains(out.String(), "my-proj") {
		t.Fatal(err, out.String())
	}
	if err := projectOff([]string{"-dir", root}, &out, home); err != nil {
		t.Fatal(err)
	}
	doc, _ = readJSONObject(path)
	env = doc["env"].(map[string]any)
	if env["HTTPS_PROXY"] != nil || env[projectMarkerKey] != nil || env["KEEP"] != "1" || doc["permissions"] == nil {
		t.Fatalf("off removed too much or too little %v", doc)
	}
	if len(loadProjectIndex(home)) != 0 {
		t.Fatal("index not cleared")
	}
}

func TestProjectOnRefusesHandWrittenProxyAndCleansEmptyFile(t *testing.T) {
	home := t.TempDir()
	ca := mitmCAPath(home)
	_ = os.MkdirAll(filepath.Dir(ca), 0o700)
	_ = os.WriteFile(ca, []byte("ca"), 0o600)
	root := t.TempDir()
	path := projectSettingsPath(root)
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	_ = os.WriteFile(path, []byte(`{"env":{"HTTPS_PROXY":"http://corp:3128"}}`), 0o600)
	var out bytes.Buffer
	if err := projectOn([]string{"-dir", root}, &out, home); err == nil {
		t.Fatal("overwrote a hand-written proxy")
	}
	_ = os.Remove(path)
	if err := projectOn([]string{"-dir", root, "-name", "abc"}, &out, home); err != nil {
		t.Fatal(err)
	}
	if err := projectOff([]string{"-all"}, &out, home); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("empty settings file left behind")
	}
	if err := projectOn([]string{"-dir", root, "-name", "bad name"}, &out, home); err == nil {
		t.Fatal("invalid name accepted")
	}
}
