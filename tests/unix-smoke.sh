#!/usr/bin/env bash
# Runs wj the way a user would on a fresh Linux or macOS machine: it really installs
# packages and rewrites config files, so it refuses to run outside CI.
set -u

if [ "${CI:-}" != "true" ]; then
    echo "Chỉ chạy trên máy CI dùng một lần: script này cài và gỡ gói thật." >&2
    exit 2
fi

repo=$(cd "$(dirname "$0")/.." && pwd)
bin="$HOME/.local/bin"
wj="$bin/wj"
work=$(mktemp -d)
log="$work/step.log"
os=$(uname -s)
failed=()
passed=0

step() {
    local name=$1
    shift
    echo "::group::$name"
    "$@" >"$log" 2>&1 </dev/null
    local code=$?
    cat "$log"
    echo "::endgroup::"
    if [ "$code" -eq 0 ]; then
        echo "PASS  $name"
        passed=$((passed + 1))
    else
        echo "FAIL  $name (exit $code)"
        failed+=("$name")
    fi
}

sh_step() { # sh_step "name" 'shell text'
    step "$1" bash -c "$FUNCS
$2"
}

# Helpers for the steps. CI runners already have ~/.local/bin on PATH, so a test of what
# a new terminal sees must start from a clean environment and read the shell's profile.
FUNCS='
base_path=/usr/bin:/bin:/usr/sbin:/sbin
if [ "$os" = Darwin ]; then user_shell=/bin/zsh; shell_flags=-lic; else user_shell=/bin/bash; shell_flags=-ic; fi
new_terminal() {
    env -i HOME="$HOME" USER="${USER:-runner}" TERM=xterm-256color LANG=C.UTF-8 PATH="$base_path" SHELL="$user_shell" \
        "$user_shell" $shell_flags "$1" 2>&1
}
pty_run() { # runs text in a real terminal, with stdin typed into it
    if [ "$os" = Darwin ]; then script -q /dev/null bash -c "$1"; else script -qec "$1" /dev/null; fi
}
'

export wj repo work bin os FUNCS
export SMOKE_KEY_VALUE="smoke-not-a-real-key"
now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
export now

# --- Install ---
sh_step "install.sh tải bản phát hành và kiểm SHA-256" '
    env PATH="$base_path" SHELL="$user_shell" sh "$repo/install.sh" --version v1.6.3 && "$wj" version'
step "build từ source hiện tại" go build -C "$repo" -o "$wj" .
sh_step "wj version" '"$wj" version'
sh_step "PATH: terminal mới thấy wj" 'new_terminal "command -v wj && wj version"'
sh_step "install.sh chạy lại không thêm PATH lần nữa" '
    env PATH="$base_path" SHELL="$user_shell" sh "$repo/install.sh" --version v1.6.3 && go build -C "$repo" -o "$wj" .
    if [ "$os" = Darwin ]; then rc="$HOME/.zshrc"; else rc="$HOME/.bashrc"; fi
    test "$(grep -cF "Added by the wj installer" "$rc")" -le 1'

# --- Menu ---
sh_step "menu: 0 thoát" 'printf "0\n" | "$wj"'
sh_step "menu: trong terminal thật, 0 thoát" 'printf "0\n" | pty_run "\"$wj\""'
sh_step "wj check" '"$wj" check'
sh_step "wj help" '"$wj" help tools && "$wj" help usage'

# --- CLI tools ---
sh_step "tools cli: xem trước" '"$wj" tools install cli -dry-run'
sh_step "tools cli: 0 huỷ, không cài gì" '
    before=$(ls -a "$HOME" | wc -l)
    WJ_GROUPS=0 bash "$repo/toolkit/unix/install-cli.sh" && test "$(ls -a "$HOME" | wc -l)" = "$before"'
sh_step "tools cli: trong terminal thật, gõ 0 huỷ" '
    printf "0\n" | pty_run "bash \"$repo/toolkit/unix/install-cli.sh\"" | tee "$work/tty.out"
    grep -qi "huy" "$work/tty.out"'
sh_step "tools cli: cài cả 4 nhóm" '
    printf "y\n" | "$wj" tools install cli -groups 1,2,3,4 | tee "$work/cli.out"
    code=${PIPESTATUS[1]}
    test "$code" -eq 0 && ! grep -q "LOI" "$work/cli.out"'
sh_step "tools cli: terminal mới có đủ lệnh" '
    new_terminal "for t in rg fd jq bat eza lazygit delta btop gh git fnm uv nvim node starship fzf zoxide claude codex agy; do command -v \$t >/dev/null || { echo thiếu \$t; failed=1; }; done; exit \${failed:-0}"'
sh_step "tools cli: agent gọi được fd, bat ngoài terminal" '
    PATH="$bin:/opt/homebrew/bin:$base_path" bash -c "fd --version && bat --version"'
sh_step "tools cli: có lệnh tips" 'new_terminal "tips" | grep -qi "fzf\|ripgrep\|rg"'
sh_step "tools cli: chạy lại không lỗi" 'printf "y\n" | "$wj" tools install cli -groups 2'

# --- Apps ---
sh_step "apps list" '"$wj" apps list'
sh_step "apps install: xem trước" '"$wj" apps install claude,codex -dry-run'
sh_step "apps install claude,codex" '"$wj" apps install claude,codex'
sh_step "apps: claude và codex chạy được trong terminal mới" 'new_terminal "claude --version && codex --version"'
sh_step "apps list thấy đã cài" '"$wj" apps list | grep -i claude'

# --- Connect, with a key that cannot work: only the written config is checked ---
sh_step "auth gateway: ghi cấu hình Claude" '
    printf "%s\n" "$SMOKE_KEY_VALUE" | "$wj" auth -auth gateway -only claude -base-url gateway.invalid -key-stdin -skip-check -models smoke-model -profile smoke >"$work/auth.out" 2>&1
    code=$?
    cat "$work/auth.out"
    test "$code" -eq 0 && grep -q gateway.invalid "$HOME/.claude/settings.json"'
sh_step "auth: key không bị in ra" '! grep -q "$SMOKE_KEY_VALUE" "$work/auth.out"'
sh_step "auth: file cấu hình chỉ chủ máy đọc được" '
    if [ "$os" = Darwin ]; then mode=$(stat -f %Lp "$HOME/.claude/settings.json"); else mode=$(stat -c %a "$HOME/.claude/settings.json"); fi
    echo "mode $mode"; test "$mode" = 600'
sh_step "auth: liệt kê profile" '"$wj" auth -list-profiles | grep smoke'
sh_step "configure codex: gateway" '
    printf "%s\n" "$SMOKE_KEY_VALUE" | "$wj" configure codex -auth gateway -base-url gateway.invalid -key-stdin -skip-check -models smoke-model
    test -s "$HOME/.codex/config.toml"'

# --- Project setup ---
sh_step "wj init -type web" '
    mkdir -p "$work/project" && cd "$work/project" && git init -q && "$wj" init -type web </dev/null && test -f AGENTS.md'

# --- Tokens, from a session file shaped like Claude Code writes ---
mkdir -p "$HOME/.claude/projects/smoke"
for model in claude-opus-4-5 claude-haiku-4-5; do
    printf '{"type":"assistant","cwd":"/w/smoke-project","timestamp":"%s","requestId":"r-%s","message":{"id":"m-%s","model":"%s","usage":{"input_tokens":1000,"output_tokens":2000,"cache_read_input_tokens":3000,"cache_creation_input_tokens":0}}}\n' \
        "$now" "$model" "$model" "$model" >>"$HOME/.claude/projects/smoke/s.jsonl"
done
sh_step "wj top -once đủ chi tiết" '
    "$wj" top -once | tee "$work/top.out"
    for want in "SESSION ĐANG CHẠY" smoke-project "2 model" claude-opus-4-5 claude-haiku-4-5 "≈USD" "Cả máy"; do
        grep -q "$want" "$work/top.out" || { echo "thiếu: $want"; exit 1; }
    done'
sh_step "wj top trực tiếp trong terminal 80 cột, Ctrl+C thoát" '
    pty_run "stty cols 80 rows 30; \"$wj\" top -interval 1s & p=\$!; sleep 3; kill -INT \$p; wait \$p; echo exit=\$?" >"$work/live.out"
    cat "$work/live.out"
    for want in smoke-project claude-haiku-4-5 "≈USD" "exit=0"; do
        grep -q "$want" "$work/live.out" || { echo "thiếu: $want"; exit 1; }
    done'
sh_step "wj usage -by project-model" '"$wj" usage -by project-model | grep claude-haiku-4-5'
sh_step "wj overlay báo rõ chỉ có trên Windows" '
    "$wj" overlay >"$work/overlay.out" 2>&1; code=$?
    cat "$work/overlay.out"
    test "$code" -ne 0 && grep -q "wj top" "$work/overlay.out"'

# --- Window manager ---
sh_step "tools wm: xem trước" '"$wj" tools install wm -dry-run'
if [ "$os" = Darwin ]; then
    sh_step "tools wm: cấu hình AeroSpace" '
        bash "$repo/toolkit/unix/install-wm.sh" && test -s "$HOME/.aerospace.toml"'
else
    sh_step "tools wm: 0 huỷ" 'WJ_WM=0 bash "$repo/toolkit/unix/install-wm.sh" && test ! -e "$HOME/.config/i3/config"'
    if command -v apt-get >/dev/null 2>&1; then
        sudo=""; [ "$(id -u)" -eq 0 ] || sudo=sudo
        $sudo apt-get update -qq >/dev/null 2>&1
        $sudo apt-get install -y -qq i3-wm >/dev/null 2>&1
        sh_step "tools wm: cấu hình i3" 'WJ_WM=i3 bash "$repo/toolkit/unix/install-wm.sh" && test -s "$HOME/.config/i3/config"'
        sh_step "tools wm: config i3 hợp lệ" 'i3 -C -c "$HOME/.config/i3/config"'
    fi
fi
sh_step "tools wm: gỡ" 'bash "$repo/toolkit/unix/uninstall-wm.sh" </dev/null'

# --- Remove ---
sh_step "tools cli: gỡ cả phần mềm" 'WJ_GROUPS="1 2 3 4" bash "$repo/toolkit/unix/uninstall-cli.sh" </dev/null'
sh_step "sau khi gỡ: terminal mới vẫn mở được, wj vẫn chạy" 'new_terminal "wj version"'
sh_step "sau khi gỡ: fd, bat do toolkit tạo đã hết" '
    for tool in fd bat; do
        link="$bin/$tool"
        ! [ -L "$link" ] || { echo "còn link $link"; exit 1; }
    done'

rm -rf "$work"
echo
echo "=== $os $(uname -m) ${SMOKE_NAME:-}: $passed đạt, ${#failed[@]} lỗi ==="
for name in "${failed[@]}"; do
    echo "LỖI: $name"
done
[ "${#failed[@]}" -eq 0 ]
