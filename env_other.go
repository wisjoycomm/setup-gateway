//go:build !windows

package main

func setEnvWindows(name, value string) error { return nil }