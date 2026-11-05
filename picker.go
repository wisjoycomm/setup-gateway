package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// parseChoices turns a typed answer into option names. Numbers are the fast
// path ("1 3", "1,3", "2-4"); "all" / "a" selects everything; names are still
// accepted so old habits and scripts keep working.
func parseChoices(answer string, names []string) ([]string, error) {
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer == "all" || answer == "a" || answer == "*" {
		return append([]string(nil), names...), nil
	}
	var picked []string
	add := func(name string) {
		if !contains(picked, name) {
			picked = append(picked, name)
		}
	}
	for _, field := range strings.FieldsFunc(answer, func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
		if lo, hi, isRange := strings.Cut(field, "-"); isRange {
			a, errA := strconv.Atoi(lo)
			b, errB := strconv.Atoi(hi)
			if errA == nil && errB == nil {
				if a < 1 || b > len(names) || a > b {
					return nil, fmt.Errorf("khoảng %q không hợp lệ; chọn từ 1 đến %d", field, len(names))
				}
				for i := a; i <= b; i++ {
					add(names[i-1])
				}
				continue
			}
		}
		if n, err := strconv.Atoi(field); err == nil {
			if n < 1 || n > len(names) {
				return nil, fmt.Errorf("không có số %d; chọn từ 1 đến %d", n, len(names))
			}
			add(names[n-1])
			continue
		}
		found := ""
		for _, name := range names {
			if strings.EqualFold(name, field) {
				found = name
			}
		}
		if found == "" {
			return nil, fmt.Errorf("không có lựa chọn %q; gõ số trong danh sách", field)
		}
		add(found)
	}
	return picked, nil
}

// pickOne prints a numbered list and returns one name ("" when the user presses
// Enter and there is no default). def is the 1-based default, 0 for none.
func pickOne(input io.Reader, out io.Writer, title string, names, labels []string, def int) (string, error) {
	printOptions(out, title, names, labels, def)
	hint := fmt.Sprintf("Gõ số [1-%d]", len(names))
	if def > 0 {
		hint += fmt.Sprintf(", Enter = %d", def)
	} else {
		hint += ", Enter = quay lại"
	}
	answer := menuAnswer(input, out, hint)
	if answer == "" {
		if def > 0 {
			return names[def-1], nil
		}
		return "", nil
	}
	picked, err := parseChoices(answer, names)
	if err != nil {
		return "", err
	}
	if len(picked) != 1 {
		return "", fmt.Errorf("chỉ chọn một số")
	}
	return picked[0], nil
}

// pickMany is pickOne for several answers, e.g. "1 3 5", "2-4" or "all".
func pickMany(input io.Reader, out io.Writer, title string, names, labels []string) ([]string, error) {
	printOptions(out, title, names, labels, 0)
	answer := menuAnswer(input, out, "Gõ số, nhiều số cách nhau dấu cách (vd 1 3, 2-4), all = tất cả, Enter = quay lại")
	if answer == "" {
		return nil, nil
	}
	return parseChoices(answer, names)
}

// pickProfile lists saved Claude connections by number and activates one.
func pickProfile(input io.Reader, out io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	names, labels := savedProfiles(home)
	if len(names) == 0 {
		return listProfiles(home, out)
	}
	name, err := pickOne(input, out, "Kết nối Claude đã lưu:", names, labels, 0)
	if err != nil || name == "" {
		return err
	}
	return runCLIWithHome([]string{"-use-profile", name}, input, out, home)
}

func savedProfiles(home string) (names, labels []string) {
	entries, _ := os.ReadDir(filepath.Join(home, ".setup-gateway", "profiles"))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		p, err := loadProfile(home, name)
		if err != nil {
			continue
		}
		label := name + "  (" + map[string]string{"gateway": "gateway " + strings.TrimSuffix(p.BaseURL, "/v1"), "oauth": "token OAuth", "api-key": "API key", "login": "đăng nhập web"}[p.Auth] + ")"
		names, labels = append(names, name), append(labels, label)
	}
	return names, labels
}

// savedGatewayProfiles lists bearer gateway profiles, reusable by Codex/OpenCode.
func savedGatewayProfiles(home string) (names, labels []string) {
	all, allLabels := savedProfiles(home)
	for i, name := range all {
		if p, err := loadProfile(home, name); err == nil && p.Auth == "gateway" && p.Header != "x-api-key" {
			names, labels = append(names, name), append(labels, allLabels[i])
		}
	}
	return names, labels
}

func printOptions(out io.Writer, title string, names, labels []string, def int) {
	if title != "" {
		fmt.Fprintln(out, title)
	}
	width := len(strconv.Itoa(len(names)))
	for i, name := range names {
		label := name
		if i < len(labels) && labels[i] != "" {
			label = labels[i]
		}
		mark := " "
		if i+1 == def {
			mark = "*"
		}
		fmt.Fprintf(out, " %s%*d. %s\n", mark, width, i+1, label)
	}
}
