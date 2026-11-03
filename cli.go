package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

func runCLI(args []string, input io.Reader, out io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return runCLIWithHome(args, input, out, home)
}

func runCLIWithHome(args []string, input io.Reader, out io.Writer, home string) error {
	flags := flag.NewFlagSet("setup-gateway", flag.ContinueOnError)
	flags.SetOutput(out)
	auth := flags.String("auth", "", "gateway, oauth (session-key), api-key, or login")
	base := flags.String("base-url", "", "custom gateway URL; accepts a bare domain")
	key := flags.String("key", "", "credential (prefer the hidden prompt, -key-stdin, or SETUP_GATEWAY_KEY)")
	keyStdin := flags.Bool("key-stdin", false, "read the credential from one line of standard input")
	header := flags.String("auth-header", "bearer", "gateway credential header: bearer or x-api-key (Claude only)")
	only := flags.String("only", "", "comma-separated clients: claude,codex,cursor,opencode,agy")
	provider := flags.String("provider-id", "gateway", "provider name used by codex and opencode")
	profile := flags.String("profile", "", "save this Claude connection as a named profile, then activate it")
	use := flags.String("use-profile", "", "activate a saved Claude profile")
	list := flags.Bool("list-profiles", false, "list saved profiles without revealing credentials")
	models := flags.String("models", "", "comma-separated gateway model IDs; replaces discovered IDs")
	skip := flags.Bool("skip-check", false, "skip the /models request (use -models for gateway model mappings)")
	install := flags.Bool("install-claude", false, "install Claude Code with its official installer if missing")
	noLogin := flags.Bool("no-login", false, "configure browser login without opening the browser yet")
	launch := flags.Bool("run", false, "start Claude Code after setup with a clean authentication environment")
	dry := flags.Bool("dry-run", false, "preview only: no writes, network, installation, or login")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected arguments; see -help")
	}
	var err error
	if *list {
		return listProfiles(home, out)
	}
	if !validName(*provider) {
		return errors.New("provider-id must contain only letters, numbers, underscore or hyphen")
	}
	if *profile != "" && !validName(*profile) {
		return errors.New("invalid profile name; use letters, numbers, underscore or hyphen")
	}
	reader := bufio.NewReader(input)
	wizard := len(args) == 0
	var p connectionProfile
	if *use != "" {
		if *auth != "" || *base != "" || *key != "" || *keyStdin || *profile != "" || *models != "" {
			return errors.New("use-profile cannot be combined with new connection credentials or model settings")
		}
		p, err = loadProfile(home, *use)
		if err != nil {
			return err
		}
		if *only == "" {
			*only = "claude"
		}
	} else {
		if *auth == "" {
			if *base != "" || *key != "" || *keyStdin || os.Getenv("SETUP_GATEWAY_KEY") != "" {
				*auth = "gateway"
			} else {
				fmt.Fprintln(out, "Claude Code setup\n  1. Custom domain / gateway\n  2. OAuth / session token (sk-ant-oat...)\n  3. Anthropic API key (direct API)\n  4. Normal browser login")
				choice := prompt(reader, out, "Method [1-4]")
				*auth = map[string]string{"1": "gateway", "2": "oauth", "3": "api-key", "4": "login"}[choice]
				if *auth == "" {
					return errors.New("choose a method from 1 to 4, or pass -auth")
				}
			}
		}
		*auth = canonicalAuth(*auth)
		if *only == "" {
			*only = "claude"
			if wizard && *auth == "gateway" {
				choice := prompt(reader, out, "Clients [claude], or all, or comma-separated names")
				if choice == "all" {
					*only = "claude,codex,cursor,opencode,agy"
				} else if choice != "" {
					*only = choice
				}
			} else if *auth == "gateway" && !wizard && *profile == "" && *header != "x-api-key" {
				// Keep the original flag-based gateway invocation compatible.
				*only = "claude,codex,agy,cursor,opencode"
			}
		}
		if *auth == "gateway" && *base == "" {
			if *keyStdin {
				return errors.New("-key-stdin requires -base-url for gateway mode")
			}
			*base = prompt(reader, out, "Custom domain (e.g. https://ai.example.com)")
		}
		if *keyStdin {
			if *key != "" {
				return errors.New("use either -key or -key-stdin, not both")
			}
			*key = prompt(reader, io.Discard, "")
		} else if *key == "" && *auth != "login" {
			*key = strings.TrimSpace(os.Getenv("SETUP_GATEWAY_KEY"))
		}
		if *key == "" && *auth != "login" {
			label := "API key / gateway token"
			if *auth == "oauth" {
				label = "OAuth token (sk-ant-oat..., NOT the browser sessionKey cookie)"
			}
			*key, err = promptSecret(input, reader, out, label)
			if err != nil {
				return err
			}
		}
		p = connectionProfile{Auth: *auth, BaseURL: *base, Credential: strings.TrimSpace(*key), Header: *header, Models: splitList(*models)}
	}
	targets := splitList(*only)
	if err := validateTargets(targets, p, *profile != "" || *use != ""); err != nil {
		return err
	}
	if (*launch || *install) && !contains(targets, "claude") {
		return errors.New("install-claude and run require the claude target")
	}
	if err := p.validate(); err != nil {
		return err
	}
	if wizard && len(targets) == 1 && targets[0] == "claude" {
		*profile = prompt(reader, out, "Save connection as profile (e.g. work, adnx; Enter = do not save)")
		if *profile != "" && !validName(*profile) {
			return errors.New("invalid profile name; use letters, numbers, underscore or hyphen")
		}
	}
	// Refuse invalid user settings before installing software or making requests.
	if contains(targets, "claude") {
		if _, err := readSettings(claudeSettingsPath(home)); err != nil {
			return err
		}
	}
	if !*dry && !*skip && (p.Auth == "gateway" || p.Auth == "api-key") {
		fmt.Fprintln(out, "Checking model access (no generation request)...")
		ids, err := discoverModels(p)
		if err != nil {
			return err
		}
		if len(p.Models) == 0 {
			p.Models = ids
		}
		fmt.Fprintf(out, "Available models: %d\n", len(ids))
	}
	if p.Auth == "gateway" && contains(targets, "claude") && !*dry && *models == "" && *use == "" && pick(p.Models, []string{"claude"}, nil) == "" && pick(p.Models, []string{"opus"}, nil) == "" && pick(p.Models, []string{"sonnet"}, nil) == "" && pick(p.Models, []string{"haiku"}, nil) == "" && pick(p.Models, []string{"fable"}, nil) == "" {
		return errors.New("no Claude model IDs found; provide gateway IDs with -models, and -skip-check if /models is unavailable")
	}
	if wizard && contains(targets, "claude") && findClaude(home) == "" {
		*install = strings.EqualFold(prompt(reader, out, "Claude Code is missing. Install from claude.ai? [y/N]"), "y")
	}
	if *install {
		if err := ensureClaude(home, *dry, out); err != nil {
			return err
		}
	}
	if !*dry && contains(targets, "claude") && (p.Auth == "login" && !*noLogin || *launch) && findClaude(home) == "" {
		return errors.New("Claude Code is not installed; rerun with -install-claude, or use -no-login for configuration only")
	}
	if *profile != "" {
		if err := saveProfile(home, *profile, p, *dry); err != nil {
			return err
		}
		fmt.Fprintf(out, "Profile: %s (%s)\n", *profile, p.Auth)
	}
	gpt := pick(p.Models, []string{"gpt"}, []string{"review"})
	for _, name := range targets {
		var line string
		var err error
		switch name {
		case "claude":
			line, err = applyClaude(home, p, *dry)
		case "codex":
			line, err = setupCodex(home, p.BaseURL, p.Credential, gpt, *provider, *dry)
		case "cursor":
			line, err = setupCursor(p.BaseURL, p.Credential, gpt, *dry)
		case "opencode":
			line, err = setupOpenCode(home, p.BaseURL, p.Credential, p.Models, *provider, *dry)
		case "agy":
			line = "SKIP - no custom endpoint; uses the signed-in account"
		}
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		fmt.Fprintf(out, "%s: %s\n", name, line)
	}
	if *dry {
		fmt.Fprintln(out, "Dry run: no files, network requests, installation, or login.")
		return nil
	}
	if contains(targets, "claude") {
		warnEnvironment(out)
		if p.Auth == "login" && !*noLogin {
			if err := runClaude(home, "auth", "login", "--claudeai"); err != nil {
				return fmt.Errorf("configuration saved, but browser login failed: %w", err)
			}
		}
		if p.Auth == "oauth" {
			fmt.Fprintln(out, "OAuth token saved. No browser login required; token validity is checked by Claude on its next request.")
		}
		if findClaude(home) == "" {
			fmt.Fprintln(out, "Claude Code is not installed yet. Rerun with -install-claude when ready.")
		}
		fmt.Fprintln(out, "Start a NEW Claude session and check /status. Existing sessions keep their previous connection.")
		if *launch {
			return runClaude(home)
		}
	}
	return nil
}

func prompt(in *bufio.Reader, out io.Writer, label string) string {
	fmt.Fprint(out, label+": ")
	line, _ := in.ReadString('\n')
	return strings.TrimSpace(line)
}

func promptSecret(input io.Reader, reader *bufio.Reader, out io.Writer, label string) (string, error) {
	if file, ok := input.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		fmt.Fprint(out, label+": ")
		value, err := term.ReadPassword(int(file.Fd()))
		fmt.Fprintln(out)
		return strings.TrimSpace(string(value)), err
	}
	return prompt(reader, out, label), nil
}

func splitList(value string) []string {
	var result []string
	for _, v := range strings.Split(value, ",") {
		if v = strings.TrimSpace(v); v != "" && !contains(result, v) {
			result = append(result, v)
		}
	}
	return result
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

func validateTargets(targets []string, p connectionProfile, profile bool) error {
	if len(targets) == 0 {
		return errors.New("select at least one client")
	}
	for _, target := range targets {
		if !contains([]string{"claude", "codex", "cursor", "opencode", "agy"}, target) {
			return fmt.Errorf("unknown target %q", target)
		}
		if (p.Auth != "gateway" || profile || p.Header == "x-api-key") && target != "claude" {
			return errors.New("OAuth, API key, login, named profiles, and x-api-key gateways require -only claude")
		}
	}
	return nil
}
