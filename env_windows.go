//go:build windows

package main

import (
	"golang.org/x/sys/windows/registry"
	"os"
	"path/filepath"
	"strings"
)

// Refresh only this process so later wizard steps can see freshly installed apps.
func refreshProcessPath() {
	paths := filepath.SplitList(os.Getenv("PATH"))
	for _, location := range []struct {
		root registry.Key
		path string
	}{
		{registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`},
		{registry.CURRENT_USER, `Environment`},
	} {
		key, err := registry.OpenKey(location.root, location.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		value, _, err := key.GetStringValue("Path")
		key.Close()
		if err != nil {
			continue
		}
		if expanded, err := registry.ExpandString(value); err == nil {
			value = expanded
		}
		for _, path := range filepath.SplitList(value) {
			found := false
			for _, old := range paths {
				if strings.EqualFold(old, path) {
					found = true
					break
				}
			}
			if path != "" && !found {
				paths = append(paths, path)
			}
		}
	}
	_ = os.Setenv("PATH", strings.Join(paths, string(os.PathListSeparator)))
}

func setEnvWindows(name, value string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(name, value)
}
