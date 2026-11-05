//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"strings"
)

func prepareConsole() func() { return func() {} }

func refreshProcessPath() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	paths := filepath.SplitList(os.Getenv("PATH"))
	for _, dir := range []string{filepath.Join(home, ".local", "bin"), filepath.Join(home, ".opencode", "bin"), filepath.Join(home, ".agy", "bin"), "/opt/homebrew/bin", "/usr/local/bin"} {
		if info, err := os.Stat(dir); err == nil && info.IsDir() && !contains(paths, dir) {
			paths = append(paths, dir)
		}
	}
	_ = os.Setenv("PATH", strings.Join(paths, string(os.PathListSeparator)))
}
