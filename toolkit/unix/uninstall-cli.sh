#!/usr/bin/env bash
# ======================================================================
#  GO CONG CU DONG LENH (macOS / Linux)
#   - Luon xoa cau hinh da them (khoi wisjoy trong ~/.bashrc, ~/.zshrc; git delta)
#   - Phan mem: chi go nhom ban chon
#  Chay:  bash uninstall.sh      Khong hoi:  WJ_GROUPS="2 4" bash uninstall.sh
# ======================================================================
set -u
MARK_A='# >>> wisjoy-cli >>>'
MARK_B='# <<< wisjoy-cli <<<'
OS=$(uname -s)
[ -x /opt/homebrew/bin/brew ] && eval "$(/opt/homebrew/bin/brew shellenv)"
[ -x /usr/local/bin/brew ] && eval "$(/usr/local/bin/brew shellenv)"
say()  { printf '\n\033[36m>> %s\033[0m\n' "$*"; }
has()  { command -v "$1" >/dev/null 2>&1; }
ask()  {
    local v="${2:-}" a=""
    if [ -n "$v" ] && [ -n "${!v:-}" ]; then printf '%s' "${!v}"; return; fi
    if [ -r /dev/tty ] && { true </dev/tty; } 2>/dev/null; then read -r -p "$1" a </dev/tty || a=""; fi
    printf '%s' "$a"
}
PM=""
if [ "$OS" = "Darwin" ]; then PM=brew
elif has apt-get; then PM=apt
elif has dnf; then PM=dnf
elif has pacman; then PM=pacman
elif has zypper; then PM=zypper
fi
SUDO=""
if [ "$(id -u)" -ne 0 ]; then has sudo && SUDO="sudo"; fi

echo
printf '\033[36m  GO CONG CU DONG LENH (%s)\033[0m\n' "$OS"

say "Xoa cau hinh da them (giu nguyen phan ban tu viet)"
for f in "$HOME/.bashrc" "$HOME/.zshrc"; do
    [ -f "$f" ] && grep -qF "$MARK_A" "$f" || continue
    cp "$f" "$f.bak-wisjoy"
    tmp=$(mktemp)
    awk -v a="$MARK_A" -v b="$MARK_B" '$0==a{skip=1;next} $0==b{skip=0;next} !skip' "$f" > "$tmp"
    printf '%s\n' "$(cat "$tmp")" > "$f"; rm -f "$tmp"
    echo "   $f"
done
rm -rf "$HOME/.local/share/wisjoy-fzf"
if has git && [ "$(git config --global core.pager)" = "delta" ]; then
    git config --global --unset core.pager
    git config --global --unset interactive.diffFilter
    git config --global --unset delta.navigate
    echo "   Git: bo delta, tro ve diff mac dinh"
fi

# nhom | ten | lenh | brew | apt | dnf | pacman | zypper | file cai tay can xoa (trong $HOME)
PKGS='
1|Starship|starship|starship|-|-|starship|starship|.local/bin/starship
1|Font JetBrainsMono Nerd|-|cask:font-jetbrains-mono-nerd-font|-|-|ttf-jetbrains-mono-nerd|-|.local/share/fonts/JetBrainsMonoNerd
1|fzf|fzf|fzf|fzf|fzf|fzf|fzf|-
1|zoxide|zoxide|zoxide|zoxide|zoxide|zoxide|zoxide|.local/bin/zoxide
1|zsh goi y lenh|-|zsh-autosuggestions|zsh-autosuggestions|zsh-autosuggestions|zsh-autosuggestions|zsh-autosuggestions|-
1|zsh to mau lenh|-|zsh-syntax-highlighting|zsh-syntax-highlighting|zsh-syntax-highlighting|zsh-syntax-highlighting|zsh-syntax-highlighting|-
2|ripgrep|rg|ripgrep|ripgrep|ripgrep|ripgrep|ripgrep|-
2|fd|fd|fd|fd-find|fd-find|fd|fd|-
2|bat|bat|bat|bat|bat|bat|bat|-
2|eza|eza|eza|eza|eza|eza|eza|.local/bin/eza
2|jq|jq|jq|jq|jq|jq|jq|-
2|lazygit|lazygit|lazygit|lazygit|-|lazygit|lazygit|.local/bin/lazygit
2|delta|delta|git-delta|git-delta|git-delta|git-delta|git-delta|-
2|btop|btop|btop|btop|btop|btop|btop|-
2|GitHub CLI|gh|gh|gh|gh|github-cli|gh|-
3|fnm|fnm|fnm|-|-|fnm|-|.local/share/fnm
3|uv|uv|uv|-|-|uv|-|.local/bin/uv .local/bin/uvx
3|Neovim|nvim|neovim|neovim|neovim|neovim|neovim|-
3|Antigravity IDE|antigravity-ide|cask:antigravity-ide|-|-|-|-|-
4|Claude Code|claude|cask:claude-code|-|-|-|-|.local/bin/claude .local/share/claude
4|Codex CLI|codex|cask:codex|-|-|-|-|.local/bin/codex
4|Antigravity CLI|agy|cask:antigravity-cli|-|-|-|-|.local/bin/agy
'
echo
echo "  Go luon phan mem? Chon nhom (vd: 2 4), Enter = KHONG go gi:"
echo "   1. Terminal (Starship, font, fzf, zoxide, plugin zsh)"
echo "   2. Tim kiem / xem file (rg, fd, bat, eza, jq, lazygit, delta, btop, gh)"
echo "   3. Lap trinh (fnm, uv, Neovim, Antigravity IDE). Git va Docker giu lai, go tay neu can"
echo "   4. Coding agent (Claude Code, Codex, agy)"
PICK=$(ask '  Nhom: ' WJ_GROUPS)
SEL=$(printf '%s' "$PICK" | tr -c '1234' ' ')
if [ -n "$(printf '%s' "$SEL" | tr -d ' ')" ]; then
    while IFS='|' read -r g name _cmd pb pa pd pp pz files; do
        [ -n "$g" ] || continue
        case " $SEL " in *" $g "*) ;; *) continue ;; esac
        echo "   Go $name"
        case "$PM" in brew) p="$pb" ;; apt) p="$pa" ;; dnf) p="$pd" ;; pacman) p="$pp" ;; zypper) p="$pz" ;; *) p="-" ;; esac
        if [ "$p" != "-" ]; then
            case "$PM" in
                brew)   if [ "${p#cask:}" != "$p" ]; then brew uninstall --cask "${p#cask:}" >/dev/null 2>&1; else brew uninstall "$p" >/dev/null 2>&1; fi ;;
                apt)    $SUDO apt-get remove -y -qq "$p" >/dev/null 2>&1 ;;
                dnf)    $SUDO dnf remove -y -q "$p" >/dev/null 2>&1 ;;
                pacman) $SUDO pacman -R --noconfirm "$p" >/dev/null 2>&1 ;;
                zypper) $SUDO zypper -n remove "$p" >/dev/null 2>&1 ;;
            esac
        fi
        if [ "$files" != "-" ]; then for x in $files; do rm -rf "${HOME:?}/$x"; done; fi
    done <<EOF
$PKGS
EOF
    has fc-cache && fc-cache -f >/dev/null 2>&1
    # Link do install-cli.sh tao cho ten rieng cua Debian/Ubuntu; goi da go thi link cung het tac dung
    case " $SEL " in *" 2 "*)
        for pair in fd:fdfind bat:batcat; do
            link="$HOME/.local/bin/${pair%%:*}"
            [ -L "$link" ] && case "$(readlink "$link")" in *"/${pair##*:}") rm -f "$link" ;; esac
        done ;;
    esac
fi
say "XONG. Mo cua so terminal moi de thay thay doi."
echo "  Ban sao file cau hinh cu: ~/.bashrc.bak-wisjoy, ~/.zshrc.bak-wisjoy"
exit 0
