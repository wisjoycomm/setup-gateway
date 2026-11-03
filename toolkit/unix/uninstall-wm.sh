#!/usr/bin/env bash
# ======================================================================
#  GO CAU HINH CUA SO KIEU i3 (AeroSpace / i3 / Hyprland)
#   - Tra lai file cau hinh ban co truoc lan chay setup-wm.sh dau tien
#     (neu truoc do chua co file thi xoa file do script tao)
#   - Giu nguyen file rieng cua ban (rieng.conf / rieng.lua)
# ======================================================================
set -u
OS=$(uname -s)
CFG="${XDG_CONFIG_HOME:-$HOME/.config}"
say() { printf '\n\033[36m>> %s\033[0m\n' "$*"; }
has() { command -v "$1" >/dev/null 2>&1; }
ask() {
    local v="${2:-}" a=""
    if [ -n "$v" ] && [ -n "${!v:-}" ]; then printf '%s' "${!v}"; return; fi
    if [ -r /dev/tty ] && { true </dev/tty; } 2>/dev/null; then read -r -p "$1" a </dev/tty || a=""; fi
    printf '%s' "$a"
}
restore() { # tra lai ban goc neu co, khong thi xoa file script tao
    local f="$1"
    [ -e "$f" ] || [ -e "$f.bak-wisjoy" ] || return 0
    if [ -e "$f.bak-wisjoy" ]; then
        rm -rf "$f"; mv "$f.bak-wisjoy" "$f"; echo "   Tra lai ban cu: $f"
    elif grep -q "setup-wm.sh" "$f" 2>/dev/null; then
        rm -f "$f"; echo "   Xoa: $f"
    fi
    rm -rf "$f.bak-wisjoy.last"
}

echo
printf '\033[36m  GO CAU HINH CUA SO KIEU i3 (%s)\033[0m\n' "$OS"
say "Tra lai cau hinh"
if [ "$OS" = "Darwin" ]; then
    restore "$HOME/.aerospace.toml"
    restore "$CFG/aerospace/aerospace.toml"
    if [ -f /etc/sudoers.d/wisjoy-pmset ]; then sudo rm -f /etc/sudoers.d/wisjoy-pmset && echo "   Xoa quyen doi che do nguon (sudoers)"; fi
    defaults write com.apple.dock autohide -bool false; killall Dock 2>/dev/null
    echo "   Hien lai Dock"
    a=$(ask "   Go luon AeroSpace? (y/N) " WJ_REMOVE_APP)
    case "$a" in y|Y|yes) has aerospace && aerospace enable off >/dev/null 2>&1
        osascript -e 'quit app "AeroSpace"' 2>/dev/null
        has brew && brew uninstall --cask aerospace >/dev/null 2>&1 && echo "   Da go AeroSpace" ;;
    esac
    echo "   'Displays have separate Spaces' van dang tat: bat lai trong System Settings > Desktop & Dock neu muon."
else
    restore "$CFG/i3/config"
    restore "$CFG/hypr/hyprland.lua"
    restore "$CFG/hypr/hyprland.conf"
    [ -n "${DISPLAY:-}" ] && has i3-msg && i3-msg reload >/dev/null 2>&1
    [ -n "${HYPRLAND_INSTANCE_SIGNATURE:-}" ] && has hyprctl && hyprctl reload >/dev/null 2>&1
fi
rm -f "$HOME/.local/bin/wj-power"
say "XONG. File rieng cua ban (rieng.conf / rieng.lua) van giu nguyen."
exit 0
