package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
)

const directHelp = `Claude Code gọi thẳng gateway của bạn, không qua proxy đo token

  wj direct             tắt proxy ở mọi project đã bật
  wj direct -dry-run    chỉ xem sẽ đổi gì, không ghi file

  Lệnh xoá HTTPS_PROXY, NODE_EXTRA_CA_CERTS, NO_PROXY mà tool đã ghi vào
  <project>/.claude/settings.local.json (như wj proxy project off -all).
  Kết nối gateway/key trong ~/.claude/settings.json giữ nguyên: Claude vẫn gọi
  thẳng lên gateway (ANTHROPIC_BASE_URL) của bạn, không qua Anthropic.

  Không tắt mitmproxy đang chạy (Ctrl+C ở cửa sổ của nó). Không đụng Codex.
  Mở phiên Claude MỚI sau khi chạy; phiên đang mở vẫn đi qua proxy.
  Bật đo lại: wj proxy project on`

func runDirect(args []string, out io.Writer, home string) error {
	flags := flag.NewFlagSet("direct", flag.ContinueOnError)
	flags.SetOutput(out)
	dry := flags.Bool("dry-run", false, "chỉ xem trước, không ghi file")
	if len(args) > 0 && contains([]string{"help", "-h", "-help", "--help"}, args[0]) {
		fmt.Fprintln(out, directHelp)
		return nil
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("cách dùng: wj direct [-dry-run]")
	}
	if *dry {
		roots := make([]string, 0)
		for root := range loadProjectIndex(home) {
			roots = append(roots, root)
		}
		sort.Strings(roots)
		if len(roots) == 0 {
			fmt.Fprintln(out, "Không có project nào đang bật proxy.")
		}
		for _, root := range roots {
			fmt.Fprintln(out, "Sẽ tắt proxy:", root)
		}
		fmt.Fprintln(out, "Dry run: không ghi file. Gateway trong ~/.claude/settings.json giữ nguyên.")
		return nil
	}
	if err := projectOff([]string{"-all"}, out, home); err != nil {
		return fmt.Errorf("tắt proxy theo project: %w", err)
	}
	fmt.Fprintln(out, "\nXong: Claude trong các project trên gọi thẳng gateway, không qua proxy. Mở phiên MỚI để áp dụng.")
	if portOpen(defaultProxyPort) {
		fmt.Fprintf(out, "Lưu ý: mitmproxy vẫn chạy ở cổng %d, không ảnh hưởng Claude nữa; tắt bằng Ctrl+C ở cửa sổ của nó.\n", defaultProxyPort)
	}
	return nil
}

func runDirectCommand(args []string, out io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return runDirect(args, out, home)
}
