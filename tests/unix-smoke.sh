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
    step "$1" bash -c "$2"
}

export wj repo work bin
export SMOKE_KEY_VALUE="smoke-not-a-real-key"
now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
export now

# --- Install ---
sh_step "install.sh tải bản phát hành và kiểm SHA-256" 'sh "$repo/install.sh" --version v1.6.3 && "$wj" version'
step "build từ source hiện tại" go build -C "$repo" -o "$wj" .
sh_step "wj version" '"$wj" version'
sh_step "PATH: bash login mới thấy wj" 'bash -lc "command -v wj"'
if command -v zsh >/dev/null 2>&1; then
    sh_step "PATH: zsh mới thấy wj" 'zsh -lic "command -v wj"'
fi

# --- Menu ---
sh_step "menu: 0 thoát" 'printf "0\n" | "$wj"'
sh_step "wj check" '"$wj" check'
sh_step "wj help" '"$wj" help tools && "$wj" help usage'

# --- CLI tools ---
sh_step "tools cli: xem trước" '"$wj" tools install cli -dry-run'
sh_step "tools cli: 0 huỷ, không cài gì" 'WJ_GROUPS=0 bash "$repo/toolkit/unix/install-cli.sh"; test $? -eq 0 && ! test -e "$HOME/.config/wj-installed-marker"'
sh_step "tools cli: cài nhóm 2,3" 'printf "y\n" | "$wj" tools install cli -groups 2,3'
sh_step "tools cli: công cụ có trong login shell" 'bash -lc "for t in rg fd jq bat eza gh; do command -v \$t || { echo thiếu \$t; exit 1; }; done"'
sh_step "tools cli: chạy lại không lỗi" 'printf "y\n" | "$wj" tools install cli -groups 2'

# --- Apps ---
sh_step "apps list" '"$wj" apps list'
sh_step "apps install: xem trước" '"$wj" apps install claude,codex -dry-run'
sh_step "apps install claude,codex" '"$wj" apps install claude,codex'
sh_step "apps: claude và codex chạy được trong login shell" 'bash -lc "claude --version && codex --version"'
sh_step "apps list thấy đã cài" '"$wj" apps list | grep -i claude'

# --- Connect, with a key that cannot work: only the written config is checked ---
sh_step "auth gateway: ghi cấu hình Claude" '
    printf "%s\n" "$SMOKE_KEY_VALUE" | "$wj" auth -auth gateway -only claude -base-url gateway.invalid -key-stdin -skip-check -models smoke-model -profile smoke >"$work/auth.out" 2>&1
    code=$?
    cat "$work/auth.out"
    test "$code" -eq 0 && grep -q gateway.invalid "$HOME/.claude/settings.json"'
sh_step "auth: key không bị in ra" '! grep -q "$SMOKE_KEY_VALUE" "$work/auth.out"'
sh_step "auth: file cấu hình chỉ chủ máy đọc được" '
    if [ "$(uname -s)" = Darwin ]; then mode=$(stat -f %Lp "$HOME/.claude/settings.json"); else mode=$(stat -c %a "$HOME/.claude/settings.json"); fi
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
    for want in smoke-project "2 model" claude-opus-4-5 claude-haiku-4-5 "≈USD" "Cả máy"; do
        grep -q "$want" "$work/top.out" || { echo "thiếu: $want"; exit 1; }
    done'
sh_step "wj usage -by project-model" '"$wj" usage -by project-model | grep claude-haiku-4-5'
sh_step "wj overlay báo rõ chỉ có trên Windows" '
    "$wj" overlay >"$work/overlay.out" 2>&1; code=$?
    cat "$work/overlay.out"
    test "$code" -ne 0 && grep -q "wj top" "$work/overlay.out"'

# --- Window manager ---
sh_step "tools wm: xem trước" '"$wj" tools install wm -dry-run'
sh_step "tools wm: 0 huỷ trên Linux" '
    [ "$(uname -s)" = Linux ] || exit 0
    WJ_WM=0 bash "$repo/toolkit/unix/install-wm.sh"; test ! -e "$HOME/.config/i3/config"'
if [ "$(uname -s)" = Linux ]; then
    sudo apt-get update -qq >/dev/null 2>&1
    sudo apt-get install -y -qq i3-wm >/dev/null 2>&1
    sh_step "tools wm: cấu hình i3" '
        WJ_WM=i3 bash "$repo/toolkit/unix/install-wm.sh" && test -s "$HOME/.config/i3/config"'
    sh_step "tools wm: config i3 hợp lệ" 'i3 -C -c "$HOME/.config/i3/config"'
else
    sh_step "tools wm: cấu hình AeroSpace" '
        bash "$repo/toolkit/unix/install-wm.sh" && test -s "$HOME/.config/aerospace/aerospace.toml"'
fi
sh_step "tools wm: gỡ" 'WJ_YES=y bash "$repo/toolkit/unix/uninstall-wm.sh" </dev/null'

# --- Remove ---
sh_step "tools cli: gỡ" 'WJ_YES=y bash "$repo/toolkit/unix/uninstall-cli.sh" </dev/null'
sh_step "wj vẫn chạy sau khi gỡ công cụ" '"$wj" version'

rm -rf "$work"
echo
echo "=== $(uname -s) $(uname -m): $passed đạt, ${#failed[@]} lỗi ==="
for name in "${failed[@]}"; do
    echo "LỖI: $name"
done
[ "${#failed[@]}" -eq 0 ]
