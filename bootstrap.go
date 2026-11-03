package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

func findClaude(home string) string {
	if path, err := exec.LookPath("claude"); err == nil {
		return path
	}
	name := "claude"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(home, ".local", "bin", name)
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return path
	}
	return ""
}

func ensureClaude(home string, dry bool, out io.Writer) error {
	if path := findClaude(home); path != "" {
		fmt.Fprintln(out, "Claude Code already installed:", path)
		return nil
	}
	if dry {
		fmt.Fprintln(out, "Would install Claude Code using the official claude.ai installer.")
		return nil
	}
	url, suffix := "https://claude.ai/install.sh", ".sh"
	if runtime.GOOS == "windows" {
		url, suffix = "https://claude.ai/install.ps1", ".ps1"
	}
	client := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" || len(via) >= 10 {
			return errors.New("invalid installer redirect")
		}
		return nil
	}}
	res, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download Claude installer: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("Claude installer returned HTTP %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, (2<<20)+1))
	if err != nil {
		return err
	}
	if len(data) == 0 || len(data) > 2<<20 {
		return errors.New("invalid Claude installer size")
	}
	f, err := os.CreateTemp("", "setup-claude-*"+suffix)
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", f.Name())
	} else {
		cmd = exec.Command("bash", f.Name())
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, out, out
	cmd.Env = cleanClaudeEnvironment(os.Environ())
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("official Claude installer failed: %w", err)
	}
	if findClaude(home) == "" {
		return errors.New("installer finished, but claude was not found; open a new terminal and rerun setup")
	}
	return nil
}

func runClaude(home string, args ...string) error {
	path := findClaude(home)
	if path == "" {
		return errors.New("Claude Code is missing; use -install-claude")
	}
	cmd := exec.Command(path, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = cleanClaudeEnvironment(os.Environ())
	return cmd.Run()
}
