package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed all:kit
var kitFS embed.FS

const (
	kitBase   = "kit/base"
	kitStacks = "kit/stacks"
)

// initStack is one kind of job. Its files live in kit/stacks/<id>/: rules.md
// goes into AGENTS.md, gitignore into .gitignore, settings.json (optional) into
// .claude/settings.json.
type initStack struct {
	id     string
	label  string
	detect []string // any of these files in the project root means this stack
}

var initStacksList = []initStack{
	{"unity", "Unity", []string{"ProjectSettings/ProjectVersion.txt"}},
	{"web", "Web (Node, React, Next...)", []string{"package.json"}},
	{"go", "Go", []string{"go.mod"}},
	{"python", "Python", []string{"pyproject.toml", "requirements.txt", "setup.py"}},
}

// initChange is one planned file result. Nothing is written until every change
// has been worked out, so a broken file stops the run before it touches anything.
type initChange struct {
	path   string
	text   string
	write  bool
	status string
}

func runInit(args []string, input io.Reader, out io.Writer) error {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(out)
	types := flags.String("type", "", "loại job, vd unity hoặc web,go (bỏ trống thì tool hỏi)")
	dry := flags.Bool("dry-run", false, "chỉ in ra sẽ làm gì, không ghi file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 1 {
		return errors.New("cách dùng: wj init [-type unity,web] [-dry-run] [thư mục]")
	}
	dir := "."
	if flags.NArg() == 1 {
		dir = flags.Arg(0)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if !isDir(abs) {
		return fmt.Errorf("không có thư mục %s", abs)
	}
	root := projectRoot(abs)
	stacks, err := chooseStacks(*types, root, input, out)
	if err != nil {
		return err
	}
	changes, err := planInit(root, stacks)
	if err != nil {
		return err
	}
	return applyInit(root, stacks, changes, *dry, out)
}

// initMenu is the numbered-menu entry: it opens from anywhere, so ask where.
func initMenu(input io.Reader, out io.Writer) error {
	dir := strings.Trim(menuAnswer(input, out, "Thư mục project (Enter = thư mục hiện tại, 0 = quay lại)"), `"' `)
	if dir == "0" {
		return errCancelled
	}
	if dir == "" {
		return runInit(nil, input, out)
	}
	return runInit([]string{dir}, input, out)
}

func chooseStacks(types, root string, input io.Reader, out io.Writer) ([]string, error) {
	names := make([]string, 0, len(initStacksList))
	for _, stack := range initStacksList {
		names = append(names, stack.id)
	}
	if types != "" {
		return parseChoices(types, names)
	}
	detected := detectStacks(root)
	labels := make([]string, 0, len(initStacksList))
	for _, stack := range initStacksList {
		label := stack.label
		if containsName(detected, stack.id) {
			label += "  (phát hiện trong project)"
		}
		labels = append(labels, label)
	}
	printOptions(out, "Project này làm gì? Rule chung luôn được cài, đây là rule thêm theo job:", names, labels, 0)
	hint := "Gõ số (vd 1 3), Enter = chỉ rule chung"
	if len(detected) > 0 {
		hint = "Gõ số (vd 1 3), Enter = dùng " + strings.Join(detected, ", ")
	}
	answer := menuAnswer(input, out, hint+", 0 = quay lại")
	if answer == "0" {
		return nil, errCancelled
	}
	if answer == "" {
		return detected, nil
	}
	return parseChoices(answer, names)
}

func detectStacks(root string) []string {
	var found []string
	for _, stack := range initStacksList {
		for _, file := range stack.detect {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(file))); err == nil {
				found = appendUnique(found, stack.id)
			}
		}
	}
	return found
}

func planInit(root string, stacks []string) ([]initChange, error) {
	var changes []initChange
	err := fs.WalkDir(kitFS, kitBase, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel := strings.TrimPrefix(path, kitBase+"/")
		if rel == "AGENTS.md" || rel == ".claude/settings.json" {
			return nil
		}
		data, err := kitFS.ReadFile(path)
		if err != nil {
			return err
		}
		change, err := planCopy(filepath.Join(root, filepath.FromSlash(rel)), string(data))
		if err != nil {
			return err
		}
		changes = append(changes, change)
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, plan := range []func(string, []string) (initChange, error){planAgents, planSettings, planGitignore} {
		change, err := plan(root, stacks)
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].path < changes[j].path })
	return changes, nil
}

// planCopy never overwrites: a file already in the project is the user's.
func planCopy(path, text string) (initChange, error) {
	old, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return initChange{path: path, text: text, write: true, status: "tạo"}, nil
	}
	if err != nil {
		return initChange{}, err
	}
	if bytes.Equal(normalizeEOL(old), normalizeEOL([]byte(text))) {
		return initChange{path: path, status: "đã đúng"}, nil
	}
	return initChange{path: path, status: "giữ nguyên (khác bản kit)"}, nil
}

func normalizeEOL(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}

// planAgents keeps wj's rules between markers, so a rerun refreshes them and
// never touches what the user wrote around them.
func planAgents(root string, stacks []string) (initChange, error) {
	path := filepath.Join(root, "AGENTS.md")
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return initChange{}, err
	}
	text := string(normalizeEOL(data))
	hasFile := err == nil
	note := ""
	if !hasFile || strings.Contains(text, blockBegin("base")) {
		base, err := kitFS.ReadFile(kitBase + "/AGENTS.md")
		if err != nil {
			return initChange{}, err
		}
		text = upsertBlock(text, "base", string(normalizeEOL(base)))
	} else {
		note = " - rule chung chưa thêm vì AGENTS.md là file của bạn"
	}
	for _, id := range stacks {
		rules, err := kitFS.ReadFile(kitStacks + "/" + id + "/rules.md")
		if err != nil {
			return initChange{}, err
		}
		text = upsertBlock(text, id, string(normalizeEOL(rules)))
	}
	if hasFile && text == string(normalizeEOL(data)) {
		return initChange{path: path, status: "đã đúng" + note}, nil
	}
	status := "tạo"
	if hasFile {
		status = "cập nhật" + note
	}
	return initChange{path: path, text: text, write: true, status: status}, nil
}

func blockBegin(id string) string { return "<!-- wj:begin " + id + " -->" }
func blockEnd(id string) string   { return "<!-- wj:end " + id + " -->" }

func upsertBlock(text, id, body string) string {
	block := blockBegin(id) + "\n" + strings.TrimSpace(body) + "\n" + blockEnd(id) + "\n"
	start := strings.Index(text, blockBegin(id))
	if start >= 0 {
		if length := strings.Index(text[start:], blockEnd(id)); length >= 0 {
			end := start + length + len(blockEnd(id))
			if end < len(text) && text[end] == '\n' {
				end++
			}
			return text[:start] + block + text[end:]
		}
	}
	if text == "" {
		return block
	}
	return strings.TrimRight(text, "\n") + "\n\n" + block
}

// planSettings only ever adds permission rules; everything else in the file stays.
func planSettings(root string, stacks []string) (initChange, error) {
	path := filepath.Join(root, ".claude", "settings.json")
	doc, err := readConfig(path)
	if err != nil {
		return initChange{}, fmt.Errorf("file %s bị lỗi cú pháp; tool không sửa nó", path)
	}
	_, statErr := os.Stat(path)
	hasFile := statErr == nil
	sources := []string{kitBase + "/.claude/settings.json"}
	for _, id := range stacks {
		sources = append(sources, kitStacks+"/"+id+"/settings.json")
	}
	added := false
	for _, source := range sources {
		data, err := kitFS.ReadFile(source)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return initChange{}, err
		}
		var kit map[string]any
		if err = json.Unmarshal(data, &kit); err != nil {
			return initChange{}, fmt.Errorf("%s: %w", source, err)
		}
		added = mergeSettings(doc, kit) || added
	}
	if hasFile && !added {
		return initChange{path: path, status: "đã đúng"}, nil
	}
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err = encoder.Encode(doc); err != nil {
		return initChange{}, err
	}
	status := "tạo"
	if hasFile {
		status = "thêm quyền"
	}
	return initChange{path: path, text: buf.String(), write: true, status: status}, nil
}

// mergeSettings reports whether it added anything. A rule already in any list
// stays where it is, so a stricter choice the user made is never loosened.
func mergeSettings(doc, kit map[string]any) bool {
	added := false
	for key, value := range kit {
		if key == "permissions" {
			continue
		}
		if _, exists := doc[key]; !exists {
			doc[key] = value
			added = true
		}
	}
	source, _ := kit["permissions"].(map[string]any)
	if source == nil {
		return added
	}
	target, _ := doc["permissions"].(map[string]any)
	if target == nil {
		target = map[string]any{}
		doc["permissions"] = target
	}
	existing := map[string]bool{}
	for _, list := range []string{"allow", "ask", "deny"} {
		values, _ := target[list].([]any)
		for _, value := range values {
			if rule, ok := value.(string); ok {
				existing[rule] = true
			}
		}
	}
	for key, value := range source {
		rules, isList := value.([]any)
		if !isList {
			if _, exists := target[key]; !exists {
				target[key] = value
				added = true
			}
			continue
		}
		current, _ := target[key].([]any)
		for _, rule := range rules {
			name, _ := rule.(string)
			if !existing[name] {
				current = append(current, rule)
				existing[name] = true
				added = true
			}
		}
		target[key] = current
	}
	return added
}

func planGitignore(root string, stacks []string) (initChange, error) {
	path := filepath.Join(root, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return initChange{}, err
	}
	hasFile := err == nil
	present := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		present[strings.TrimSpace(line)] = true
	}
	wanted := []string{".tmp/", ".claude/settings.local.json"}
	for _, id := range stacks {
		extra, err := kitFS.ReadFile(kitStacks + "/" + id + "/gitignore")
		if err != nil {
			return initChange{}, err
		}
		wanted = append(wanted, strings.Fields(string(extra))...)
	}
	var missing []string
	for _, line := range wanted {
		if !present[line] {
			present[line] = true
			missing = append(missing, line)
		}
	}
	if len(missing) == 0 {
		return initChange{path: path, status: "đã đúng"}, nil
	}
	text := string(data)
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if text != "" {
		text += "\n"
	}
	text += "# wj init\n" + strings.Join(missing, "\n") + "\n"
	status := "tạo"
	if hasFile {
		status = fmt.Sprintf("thêm %d dòng", len(missing))
	}
	return initChange{path: path, text: text, write: true, status: status}, nil
}

func applyInit(root string, stacks []string, changes []initChange, dry bool, out io.Writer) error {
	jobs := "chỉ rule chung"
	if len(stacks) > 0 {
		jobs = "rule chung + " + strings.Join(stacks, ", ")
	}
	if dry {
		fmt.Fprintf(out, "Xem trước, chưa ghi gì: %s (%s)\n", root, jobs)
	} else {
		fmt.Fprintf(out, "Setup project: %s (%s)\n", root, jobs)
	}
	for _, change := range changes {
		if change.write && !dry {
			if err := os.MkdirAll(filepath.Dir(change.path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(change.path, []byte(change.text), 0o644); err != nil {
				return err
			}
		}
		rel, err := filepath.Rel(root, change.path)
		if err != nil {
			rel = change.path
		}
		fmt.Fprintf(out, "  %-62s %s\n", filepath.ToSlash(rel), change.status)
	}
	if !isGitRepo(root) {
		fmt.Fprintln(out, "\nProject chưa có git. Worker Orca cần git: chạy git init rồi commit các file này.")
		return nil
	}
	if !dry {
		fmt.Fprintln(out, "\nCommit các file này trước khi chạy worker Orca: worktree chỉ thấy file đã commit.")
	}
	return nil
}

func isGitRepo(root string) bool {
	_, err := os.Stat(filepath.Join(root, ".git"))
	return err == nil
}
