package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed toolkit/windows/*.ps1 toolkit/unix/*.sh
var bundledToolkit embed.FS

func runApp(args []string, input io.Reader, out io.Writer) error {
	if len(args) == 0 {
		return mainMenu(input, out)
	}
	switch args[0] {
	case "auth":
		return runCLI(args[1:], input, out)
	case "tools":
		return runToolkit(args[1:], input, out)
	case "onboard":
		if len(args) != 1 {
			return errors.New("onboard is interactive and accepts no options; use tools or auth for scripted setup")
		}
		return onboard(input, out)
	case "help", "-h", "--help", "-help":
		fmt.Fprintln(out, "Wisjoy setup-gateway: one CLI for a new development machine\n\n  setup-gateway                         interactive menu\n  setup-gateway onboard                 new-machine wizard\n  setup-gateway auth                    choose a Claude authentication method\n  setup-gateway tools list              show bundled tool groups\n  setup-gateway tools install cli       terminal, utilities, runtimes, coding agents\n  setup-gateway tools install wm        Komorebi / AeroSpace / i3 / Hyprland\n  setup-gateway tools uninstall cli     remove shell integration and selected apps\n  setup-gateway tools uninstall wm      remove window-manager setup\n  setup-gateway tools install cli -groups 1,2,4 -dry-run\n  setup-gateway -use-profile work -run  activate a Claude connection and launch\n\nAuthentication options (also accepted without the auth subcommand):")
		return runCLI([]string{"-help"}, input, out)
	default:
		return runCLI(args, input, out)
	}
}

// Do not buffer ahead across menu/child-process boundaries: the child owns stdin.
func menuAnswer(input io.Reader, out io.Writer, label string) string {
	fmt.Fprint(out, label+": ")
	var line []byte
	var b [1]byte
	for {
		n, err := input.Read(b[:])
		if n > 0 {
			if b[0] == '\n' {
				break
			}
			line = append(line, b[0])
		}
		if err != nil {
			break
		}
	}
	return strings.TrimSpace(string(line))
}

func mainMenu(input io.Reader, out io.Writer) error {
	fmt.Fprintln(out, "Wisjoy - Cai dat may moi\n  1. Cai dat may moi (cong cu + Claude)\n  2. Ket noi Claude: OAuth / API key / gateway / login\n  3. Cai bo cong cu dong lenh\n  4. Cai quan ly cua so\n  5. Go bo cong cu dong lenh\n  6. Go quan ly cua so\n  7. Xem / chon profile Claude da luu\n  0. Thoat")
	switch menuAnswer(input, out, "Chon [0-7]") {
	case "", "0":
		return nil
	case "1":
		return onboard(input, out)
	case "2":
		return runCLI(nil, input, out)
	case "3":
		return runToolkit([]string{"install", "cli"}, input, out)
	case "4":
		return runToolkit([]string{"install", "wm"}, input, out)
	case "5":
		return runToolkit([]string{"uninstall", "cli"}, input, out)
	case "6":
		return runToolkit([]string{"uninstall", "wm"}, input, out)
	case "7":
		if err := runCLI([]string{"-list-profiles"}, input, out); err != nil {
			return err
		}
		name := menuAnswer(input, out, "Profile de kich hoat (Enter = bo qua)")
		if name == "" {
			return nil
		}
		return runCLI([]string{"-use-profile", name}, input, out)
	default:
		return errors.New("choose a menu item from 0 to 7")
	}
}

func onboard(input io.Reader, out io.Writer) error {
	fmt.Fprintln(out, "New-machine setup: select only the components you want.")
	if strings.EqualFold(menuAnswer(input, out, "Install command-line toolkit? [y/N]"), "y") {
		if err := runToolkit([]string{"install", "cli"}, input, out); err != nil {
			return err
		}
	}
	if strings.EqualFold(menuAnswer(input, out, "Install/configure window manager? [y/N]"), "y") {
		if err := runToolkit([]string{"install", "wm"}, input, out); err != nil {
			return err
		}
	}
	if strings.EqualFold(menuAnswer(input, out, "Configure Claude authentication? [Y/n]"), "n") {
		return nil
	}
	return runCLI(nil, input, out)
}

func toolkitScript(platform, action, module string) (string, error) {
	if action != "install" && action != "uninstall" {
		return "", errors.New("tool action must be install or uninstall")
	}
	if module != "cli" && module != "wm" {
		return "", errors.New("tool module must be cli or wm")
	}
	switch platform {
	case "windows":
		return "toolkit/windows/" + action + "-" + module + ".ps1", nil
	case "darwin", "linux":
		return "toolkit/unix/" + action + "-" + module + ".sh", nil
	default:
		return "", fmt.Errorf("toolkit is not supported on %s", platform)
	}
}

func toolkitList(out io.Writer) {
	fmt.Fprintln(out, "Bundled tools from Bo_cong_cu:\n  cli group 1: terminal, Starship, Nerd Font, fzf, zoxide; PowerShell/Clink on Windows\n  cli group 2: ripgrep, fd, bat, eza, jq, lazygit, delta, btop, GitHub CLI\n  cli group 3: Git, fnm/Node, uv/Python, Neovim, Antigravity IDE; optional Docker\n  cli group 4: Claude Code, Codex CLI, Antigravity CLI\n  wm Windows: Komorebi + whkd, i3-style keys, monitor workspaces and bars\n  wm macOS: AeroSpace and i3-style keys\n  wm Linux: configure an existing i3/Hyprland installation\n\nBoth modules include uninstall. Package availability depends on your OS.\nUse -dry-run to preview an action without extracting or executing scripts.")
}

func runToolkit(args []string, input io.Reader, out io.Writer) error {
	if len(args) == 0 || args[0] == "list" || args[0] == "-help" {
		toolkitList(out)
		return nil
	}
	if len(args) < 2 {
		return errors.New("usage: setup-gateway tools install|uninstall cli|wm [-groups 1,2,4] [-dry-run]")
	}
	action, module := args[0], args[1]
	script, err := toolkitScript(runtime.GOOS, action, module)
	if err != nil {
		return err
	}
	flags := flag.NewFlagSet("tools", flag.ContinueOnError)
	flags.SetOutput(out)
	groups := flags.String("groups", "", "CLI package groups, comma-separated: 1 terminal, 2 utilities, 3 development, 4 agents")
	dry := flags.Bool("dry-run", false, "show the bundled action without running it")
	if err := flags.Parse(args[2:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected tools arguments")
	}
	selected := splitList(*groups)
	if len(selected) > 0 && module != "cli" {
		return errors.New("-groups applies only to the cli module")
	}
	for _, group := range selected {
		if !contains([]string{"1", "2", "3", "4"}, group) {
			return errors.New("groups must be a comma-separated subset of 1,2,3,4")
		}
	}
	fmt.Fprintf(out, "%s %s on %s using bundled %s\n", action, module, runtime.GOOS, script)
	if module == "cli" {
		fmt.Fprintln(out, "Updates the toolkit's marked shell-profile block, command aliases, terminal styling and Git defaults.")
	} else {
		fmt.Fprintln(out, "Changes window-manager configuration, shortcuts, workspace layout and desktop/taskbar integration.")
	}
	if len(selected) > 0 {
		fmt.Fprintln(out, "Selected CLI groups:", strings.Join(selected, ", "))
	}
	if *dry {
		fmt.Fprintln(out, "Dry run: no extraction, package installs, configuration changes or processes.")
		return nil
	}
	if !strings.EqualFold(menuAnswer(input, out, "Continue? [y/N]"), "y") {
		fmt.Fprintln(out, "Cancelled.")
		return nil
	}
	data, err := bundledToolkit.ReadFile(script)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "wisjoy-toolkit-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, filepath.Base(script))
	if runtime.GOOS == "windows" {
		data = append([]byte{0xef, 0xbb, 0xbf}, data...)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", path)
	} else {
		cmd = exec.Command("bash", path)
	}
	cmd.Env = cleanClaudeEnvironment(os.Environ())
	// The CLI flags own group selection; do not silently inherit an unattended selection.
	var env []string
	for _, value := range cmd.Env {
		if !strings.HasPrefix(strings.ToUpper(value), "WJ_") {
			env = append(env, value)
		}
	}
	cmd.Env = append(env, "WJ_GROUPS="+strings.Join(selected, " "))
	cmd.Stdin, cmd.Stdout, cmd.Stderr = input, out, out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s did not complete; review package results above: %w", action, module, err)
	}
	fmt.Fprintln(out, "Toolkit action completed. Open a new terminal for PATH/profile changes.")
	return nil
}
