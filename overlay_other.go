//go:build !windows

package main

import (
	"errors"
	"io"
)

func runOverlay(_ []string, _ io.Writer, _ string) error {
	return errors.New("overlay hiện chỉ có trên Windows; ở đây hãy dùng wj top")
}
