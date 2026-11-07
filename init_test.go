package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runInitIn(t *testing.T, root string, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	if err := runInit(append(args, root), strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestEveryStackHasRulesAndGitignore(t *testing.T) {
	for _, stack := range initStacksList {
		for _, file := range []string{"rules.md", "gitignore"} {
			if _, err := kitFS.ReadFile(kitStacks + "/" + stack.id + "/" + file); err != nil {
				t.Errorf("%s: %v", stack.id, err)
			}
		}
	}
}

func TestInitFreshProjectInstallsKitAndChosenRules(t *testing.T) {
	root := t.TempDir()
	runInitIn(t, root, "-type", "unity")
	for _, rel := range []string{
		"CLAUDE.md", "PLAN.md", "PLAN-ARCHIVE.md", ".claude/settings.json",
		".claude/skills/orca-orchestrator/SKILL.md",
		".claude/skills/orca-stateless-worker/scripts/fast_verify.ps1",
		".agents/skills/orca-orchestrator/scripts/plan.ps1",
		".agents/skills/orca-stateless-worker/SKILL.md",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Errorf("%s missing", rel)
		}
	}
	agents := readText(t, filepath.Join(root, "AGENTS.md"))
	for _, want := range []string{"## Read less", "## Unity", ".meta"} {
		if !strings.Contains(agents, want) {
			t.Errorf("AGENTS.md lacks %q", want)
		}
	}
	if strings.Contains(agents, "## Web") {
		t.Error("web rules were not chosen")
	}
	ignore := readText(t, filepath.Join(root, ".gitignore"))
	if !strings.Contains(ignore, ".tmp/") || !strings.Contains(ignore, "/[Ll]ibrary/") {
		t.Errorf("gitignore = %q", ignore)
	}
}

func TestInitKitIsGeneralWithoutChosenStack(t *testing.T) {
	root := t.TempDir()
	runInitIn(t, root)
	agents := readText(t, filepath.Join(root, "AGENTS.md"))
	settings := readText(t, filepath.Join(root, ".claude", "settings.json"))
	plan := readText(t, filepath.Join(root, "PLAN.md"))
	for _, unwanted := range []string{"Unity", "*.prefab", ".next"} {
		if strings.Contains(agents, unwanted) {
			t.Errorf("base AGENTS.md mentions %q", unwanted)
		}
	}
	if strings.Contains(settings, "npm") || strings.Contains(settings, "go test") {
		t.Errorf("base settings has stack rules:\n%s", settings)
	}
	if strings.Contains(plan, "**T-") {
		t.Errorf("PLAN.md has sample tasks:\n%s", plan)
	}
}

func TestInitDetectsStackWhenNoAnswer(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "ProjectSettings", "ProjectVersion.txt"), "m_EditorVersion: 6000.0")
	writeTestFile(t, filepath.Join(root, "package.json"), "{}")
	runInitIn(t, root)
	agents := readText(t, filepath.Join(root, "AGENTS.md"))
	if !strings.Contains(agents, "## Unity") || !strings.Contains(agents, "## Web") {
		t.Errorf("detected stacks missing:\n%s", agents)
	}
}

func TestInitNeverOverwritesUserFiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "CLAUDE.md"), "mine")
	writeTestFile(t, filepath.Join(root, "PLAN.md"), "my plan")
	writeTestFile(t, filepath.Join(root, "AGENTS.md"), "my rules\n")
	writeTestFile(t, filepath.Join(root, ".gitignore"), ".tmp/\nkeep-me\n")
	writeTestFile(t, filepath.Join(root, ".claude", "skills", "orca-orchestrator", "SKILL.md"), "edited")
	writeTestFile(t, filepath.Join(root, ".claude", "settings.json"),
		`{"theme":"dark","permissions":{"deny":["Bash(npm install)"],"allow":["Bash(mine)"]}}`)
	output := runInitIn(t, root, "-type", "web")

	if readText(t, filepath.Join(root, "CLAUDE.md")) != "mine" || readText(t, filepath.Join(root, "PLAN.md")) != "my plan" {
		t.Error("user file overwritten")
	}
	if readText(t, filepath.Join(root, ".claude", "skills", "orca-orchestrator", "SKILL.md")) != "edited" {
		t.Error("edited skill overwritten")
	}
	if !strings.Contains(output, "giữ nguyên (khác bản kit)") {
		t.Errorf("difference not reported:\n%s", output)
	}
	agents := readText(t, filepath.Join(root, "AGENTS.md"))
	if !strings.HasPrefix(agents, "my rules\n") || !strings.Contains(agents, "## Web") {
		t.Errorf("AGENTS.md = %q", agents)
	}
	if strings.Contains(agents, "## Read less") {
		t.Error("base rules forced into a user's AGENTS.md")
	}
	ignore := readText(t, filepath.Join(root, ".gitignore"))
	if !strings.HasPrefix(ignore, ".tmp/\nkeep-me\n") || strings.Count(ignore, ".tmp/") != 1 {
		t.Errorf("gitignore = %q", ignore)
	}

	var doc struct {
		Theme       string
		Permissions struct{ Allow, Deny []string }
	}
	if err := json.Unmarshal([]byte(readText(t, filepath.Join(root, ".claude", "settings.json"))), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Theme != "dark" || !containsName(doc.Permissions.Allow, "Bash(mine)") || !containsName(doc.Permissions.Allow, "Bash(git status)") {
		t.Errorf("settings = %+v", doc)
	}
	if containsName(doc.Permissions.Allow, "Bash(npm install)") {
		t.Error("user's deny rule was loosened")
	}
}

func TestInitRerunKeepsUserTextAndChangesNothing(t *testing.T) {
	root := t.TempDir()
	runInitIn(t, root, "-type", "go")
	path := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(path, []byte(readText(t, path)+"\n## My team\n- keep this\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := readText(t, path)
	settingsBefore := readText(t, filepath.Join(root, ".claude", "settings.json"))
	runInitIn(t, root, "-type", "go")
	if got := readText(t, path); got != before {
		t.Errorf("rerun changed AGENTS.md:\n%s", got)
	}
	if got := readText(t, filepath.Join(root, ".claude", "settings.json")); got != settingsBefore {
		t.Error("rerun changed settings.json")
	}

	runInitIn(t, root, "-type", "web")
	got := readText(t, path)
	if !strings.Contains(got, "## My team") || !strings.Contains(got, "## Go") || !strings.Contains(got, "## Web") {
		t.Errorf("adding a stack lost text:\n%s", got)
	}
	if strings.Count(got, "## Read less") != 1 {
		t.Error("base block duplicated")
	}
}

func TestInitRefreshesOldBlockInPlace(t *testing.T) {
	text := "top\n" + blockBegin("go") + "\nold\n" + blockEnd("go") + "\nbottom\n"
	got := upsertBlock(text, "go", "new")
	want := "top\n" + blockBegin("go") + "\nnew\n" + blockEnd("go") + "\nbottom\n"
	if got != want {
		t.Errorf("got %q", got)
	}
}

func TestInitDryRunWritesNothing(t *testing.T) {
	root := t.TempDir()
	output := runInitIn(t, root, "-dry-run", "-type", "unity")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("dry run wrote %d entries", len(entries))
	}
	if !strings.Contains(output, "AGENTS.md") || !strings.Contains(output, "tạo") {
		t.Errorf("output:\n%s", output)
	}
}

func TestInitBrokenSettingsStopsBeforeWriting(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".claude", "settings.json")
	writeTestFile(t, path, "{ broken")
	var out bytes.Buffer
	if err := runInit([]string{"-type", "go", root}, strings.NewReader(""), &out); err == nil {
		t.Fatal("expected an error")
	}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
		t.Error("files were written before the error")
	}
	if readText(t, path) != "{ broken" {
		t.Error("broken file was changed")
	}
}

func TestInitUsesGitRootFromSubfolder(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "src", "deep")
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	runInitIn(t, sub)
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); err != nil {
		t.Error("AGENTS.md should be at the git root")
	}
	if _, err := os.Stat(filepath.Join(sub, "AGENTS.md")); err == nil {
		t.Error("AGENTS.md should not be in the subfolder")
	}
}
