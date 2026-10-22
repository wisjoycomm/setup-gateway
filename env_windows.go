//go:build windows

package main

import "golang.org/x/sys/windows/registry"

func setEnvWindows(name, value string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(name, value)
}
