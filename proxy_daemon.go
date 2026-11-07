package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The background proxy is mitmdump started detached, so it outlives the terminal
// that started it. Its pid is kept next to the usage log.

func proxyPidPath(home string) string { return filepath.Join(proxyDir(home), "mitmdump.pid") }

// ensureProxyRunning starts the metering proxy in the background unless
// something already listens on the default port.
func ensureProxyRunning(home string, out io.Writer) error {
	if portOpen(defaultProxyPort) {
		fmt.Fprintf(out, "   proxy đã chạy ở cổng %d\n", defaultProxyPort)
		return nil
	}
	bin, err := ensureMitm(home, "mitmdump", true, strings.NewReader(""), out)
	if err != nil {
		return err
	}
	if err := writeAddon(home); err != nil {
		return err
	}
	logPath := filepath.Join(proxyDir(home), "mitmdump.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer logFile.Close()
	command := mitmCommand(bin, home, defaultProxyPort, defaultWebPort)
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("không bật được mitmproxy: %w", err)
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	_ = os.WriteFile(proxyPidPath(home), []byte(strconv.Itoa(pid)), 0o600)
	deadline := time.Now().Add(30 * time.Second)
	for !portOpen(defaultProxyPort) {
		if !processAlive(pid) {
			return fmt.Errorf("mitmproxy tắt ngay khi bật; xem %s", logPath)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("mitmproxy chưa mở cổng %d sau 30 giây; xem %s", defaultProxyPort, logPath)
		}
		time.Sleep(300 * time.Millisecond)
	}
	fmt.Fprintf(out, "   proxy chạy nền ở cổng %d (pid %d); tắt: wj proxy stop\n", defaultProxyPort, pid)
	return nil
}

// stopProxy ends a background proxy wj started. A proxy started in a terminal
// (wj proxy start) has no pid file and is left alone.
func stopProxy(home string, out io.Writer) error {
	data, err := os.ReadFile(proxyPidPath(home))
	if errors.Is(err, os.ErrNotExist) {
		if portOpen(defaultProxyPort) {
			return fmt.Errorf("proxy ở cổng %d không do wj bật nền; tắt bằng Ctrl+C ở cửa sổ của nó", defaultProxyPort)
		}
		fmt.Fprintln(out, "Proxy không chạy.")
		return nil
	}
	if err != nil {
		return err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		_ = os.Remove(proxyPidPath(home))
		return errors.New("file pid hỏng; đã xoá")
	}
	if processAlive(pid) {
		if err := killProcess(pid); err != nil {
			return fmt.Errorf("không tắt được proxy (pid %d): %w", pid, err)
		}
	}
	_ = os.Remove(proxyPidPath(home))
	for i := 0; i < 50 && portOpen(defaultProxyPort); i++ {
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Fprintln(out, "Đã tắt proxy chạy nền.")
	return nil
}

func restartProxy(home string, out io.Writer) error {
	if err := stopProxy(home, out); err != nil {
		return err
	}
	return ensureProxyRunning(home, out)
}
