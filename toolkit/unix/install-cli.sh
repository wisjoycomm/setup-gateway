#!/usr/bin/env bash
# ======================================================================
#  CAI CONG CU DONG LENH cho macOS va Linux (bash / zsh)
#   - Chon nhom: terminal dep, tim kiem/xem file, lap trinh backend, coding agent
#   - macOS: Homebrew | Linux: apt / dnf / pacman / zypper, thieu thi tai ban chinh thuc
#   - Bo qua phan mem da co, chay lai bao nhieu lan cung duoc
#   - Cau hinh nam trong 1 khoi co danh dau o ~/.zshrc / ~/.bashrc
#  Chay:  bash setup.sh        (go: bash uninstall.sh)
#  Khong hoi gi:  WJ_GROUPS="1 2 4" WJ_DOCKER=n bash setup.sh
# ======================================================================
set -u
set -o pipefail   # loi tai file (curl) thi bao loi, khong bao OK nham

MARK_A='# >>> wisjoy-cli >>>'
MARK_B='# <<< wisjoy-cli <<<'
OS=$(uname -s)
ARCH=$(uname -m)
BIN="$HOME/.local/bin"
mkdir -p "$BIN"
export PATH="$BIN:$HOME/.local/share/fnm:$PATH"
[ -x /opt/homebrew/bin/brew ] && eval "$(/opt/homebrew/bin/brew shellenv)"
[ -x /usr/local/bin/brew ] && eval "$(/usr/local/bin/brew shellenv)"

say()  { printf '\n\033[36m>> %s\033[0m\n' "$*"; }
ok()   { printf '\033[32m%s\033[0m\n' "$*"; }
warn() { printf '\033[33m%s\033[0m\n' "$*"; }
has()  { command -v "$1" >/dev/null 2>&1; }
ask()  { # ask "cau hoi" "bien moi truong de tra loi san"
    local v="${2:-}" a=""
    if [ -n "$v" ] && [ -n "${!v:-}" ]; then printf '%s' "${!v}"; return; fi
    if [ -r /dev/tty ] && { true </dev/tty; } 2>/dev/null; then read -r -p "$1" a </dev/tty || a=""; fi
    printf '%s' "$a"
}
RESULTS=""
note() { RESULTS="${RESULTS}$(printf '   %-26s %s' "$1" "$2")"$'\n'; }

# ---------------- Trinh quan ly goi ----------------
PM=""
if [ "$OS" = "Darwin" ]; then PM=brew
elif has apt-get; then PM=apt
elif has dnf; then PM=dnf
elif has pacman; then PM=pacman
elif has zypper; then PM=zypper
fi
SUDO=""
if [ "$(id -u)" -ne 0 ]; then has sudo && SUDO="sudo"; fi

PM_READY=0
pm_prepare() {
    [ "$PM_READY" = 1 ] && return 0
    PM_READY=1
    case "$PM" in
        brew)
            if ! has brew; then
                say "Cai Homebrew (se hoi mat khau may Mac)"
                /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
                [ -x /opt/homebrew/bin/brew ] && eval "$(/opt/homebrew/bin/brew shellenv)"
                [ -x /usr/local/bin/brew ] && eval "$(/usr/local/bin/brew shellenv)"
            fi ;;
        apt)    $SUDO apt-get update -qq >/dev/null 2>&1
                $SUDO apt-get install -y -qq curl ca-certificates unzip tar xz-utils fontconfig >/dev/null 2>&1 ;;
        dnf)    $SUDO dnf install -y -q curl ca-certificates unzip tar xz fontconfig >/dev/null 2>&1 ;;
        pacman) $SUDO pacman -Sy --needed --noconfirm curl ca-certificates unzip tar xz fontconfig >/dev/null 2>&1 ;;
        zypper) $SUDO zypper -n install curl ca-certificates unzip tar xz fontconfig >/dev/null 2>&1 ;;
    esac
}
pm_install() { # pm_install goi   (goi "cask:ten" = brew cask)
    local p="$1"
    [ -z "$p" ] || [ "$p" = "-" ] && return 1
    case "$PM" in
        brew)   if [ "${p#cask:}" != "$p" ]; then brew install --cask "${p#cask:}" >/dev/null 2>&1; else brew install "$p" >/dev/null 2>&1; fi ;;
        apt)    $SUDO apt-get install -y -qq "$p" >/dev/null 2>&1 ;;
        dnf)    $SUDO dnf install -y -q "$p" >/dev/null 2>&1 ;;
        pacman) $SUDO pacman -S --needed --noconfirm "$p" >/dev/null 2>&1 ;;
        zypper) $SUDO zypper -n install "$p" >/dev/null 2>&1 ;;
        *)      return 1 ;;
    esac
}

# ---------------- Cai tu trang chinh thuc (khi kho goi khong co) ----------------
lin_arch() { case "$ARCH" in x86_64|amd64) echo x86_64 ;; aarch64|arm64) echo aarch64 ;; *) echo "$ARCH" ;; esac; }
fb_starship() { curl -fsSL https://starship.rs/install.sh | sh -s -- -y -b "$BIN" >/dev/null 2>&1; }
fb_zoxide()   { curl -fsSL https://raw.githubusercontent.com/ajeetdsouza/zoxide/main/install.sh | sh >/dev/null 2>&1; }
fb_eza() {
    [ "$OS" = "Linux" ] || return 1
    curl -fsSL "https://github.com/eza-community/eza/releases/latest/download/eza_$(lin_arch)-unknown-linux-gnu.tar.gz" | tar xz -C "$BIN" 2>/dev/null
}
fb_lazygit() {
    [ "$OS" = "Linux" ] || return 1
    local v a
    v=$(curl -fsSL https://api.github.com/repos/jesseduffield/lazygit/releases/latest 2>/dev/null | sed -n 's/.*"tag_name": *"v\([^"]*\)".*/\1/p' | head -1)
    [ -z "$v" ] && v=$(curl -fsSL -o /dev/null -w '%{url_effective}' https://github.com/jesseduffield/lazygit/releases/latest 2>/dev/null | sed 's#.*/v##')
    a=$(lin_arch); [ "$a" = "aarch64" ] && a=arm64
    [ -n "$v" ] && curl -fsSL "https://github.com/jesseduffield/lazygit/releases/download/v${v}/lazygit_${v}_Linux_${a}.tar.gz" | tar xz -C "$BIN" lazygit 2>/dev/null
}
fb_fnm() { curl -fsSL https://fnm.vercel.app/install | bash -s -- --skip-shell --install-dir "$HOME/.local/share/fnm" >/dev/null 2>&1; }
fb_uv()  { curl -LsSf https://astral.sh/uv/install.sh | env UV_NO_MODIFY_PATH=1 sh >/dev/null 2>&1; }
fb_font() {
    [ "$OS" = "Linux" ] || return 1
    has fc-cache || return 1
    local d="$HOME/.local/share/fonts/JetBrainsMonoNerd"
    mkdir -p "$d"
    curl -fsSL https://github.com/ryanoasis/nerd-fonts/releases/latest/download/JetBrainsMono.tar.xz | tar xJ -C "$d" 2>/dev/null && fc-cache -f >/dev/null 2>&1
}
MANUAL=""
fb_agide() { MANUAL="antigravity.google/download"; return 3; }   # 3 = can tai tay
fb_docker() {
    [ "$OS" = "Linux" ] || return 1
    curl -fsSL https://get.docker.com | $SUDO sh >/dev/null 2>&1 || return 1
    [ "$(id -u)" -ne 0 ] && $SUDO usermod -aG docker "$USER" 2>/dev/null
    return 0
}
fb_claude() { curl -fsSL https://claude.ai/install.sh | bash >/dev/null 2>&1; }
fb_codex()  { curl -fsSL https://chatgpt.com/codex/install.sh | sh >/dev/null 2>&1; }
fb_agy()    { curl -fsSL https://antigravity.google/cli/install.sh | bash >/dev/null 2>&1; }

# ---------------- Kiem tra da co chua ----------------
font_ok() {
    ls "$HOME/Library/Fonts/"JetBrainsMonoNerdFont* >/dev/null 2>&1 && return 0
    has fc-list && fc-list 2>/dev/null | grep -qi "JetBrainsMono Nerd" && return 0
    return 1
}
ZSH_PLUG_DIRS="/opt/homebrew/share /usr/local/share /usr/share /usr/share/zsh/plugins"
zplug_ok() { local d; for d in $ZSH_PLUG_DIRS; do [ -f "$d/$1/$1.zsh" ] && return 0; done; return 1; }
is_ok() { # is_ok "lenh1 lenh2" | font | zplug:ten
    case "$1" in
        font) font_ok ;;
        zplug:*) zplug_ok "${1#zplug:}" ;;
        *) local c; for c in $1; do has "$c" && return 0; done; return 1 ;;
    esac
}

# ---------------- Danh sach ----------------
# nhom | ten | kiem tra | brew | apt | dnf | pacman | zypper | cai tay
PKGS='
1|Starship (prompt dep)|starship|starship|-|-|starship|starship|fb_starship
1|Font JetBrainsMono Nerd|font|cask:font-jetbrains-mono-nerd-font|-|-|ttf-jetbrains-mono-nerd|-|fb_font
1|fzf (tim mo)|fzf|fzf|fzf|fzf|fzf|fzf|-
1|zoxide (cd thong minh)|zoxide|zoxide|zoxide|zoxide|zoxide|zoxide|fb_zoxide
1|zsh goi y lenh|zplug:zsh-autosuggestions|zsh-autosuggestions|zsh-autosuggestions|zsh-autosuggestions|zsh-autosuggestions|zsh-autosuggestions|-
1|zsh to mau lenh|zplug:zsh-syntax-highlighting|zsh-syntax-highlighting|zsh-syntax-highlighting|zsh-syntax-highlighting|zsh-syntax-highlighting|zsh-syntax-highlighting|-
2|ripgrep (rg)|rg|ripgrep|ripgrep|ripgrep|ripgrep|ripgrep|-
2|fd|fd fdfind|fd|fd-find|fd-find|fd|fd|-
2|bat|bat batcat|bat|bat|bat|bat|bat|-
2|eza (ls dep)|eza|eza|eza|eza|eza|eza|fb_eza
2|jq|jq|jq|jq|jq|jq|jq|-
2|lazygit|lazygit|lazygit|lazygit|-|lazygit|lazygit|fb_lazygit
2|delta (git diff dep)|delta|git-delta|git-delta|git-delta|git-delta|git-delta|-
2|btop|btop|btop|btop|btop|btop|btop|-
2|GitHub CLI (gh)|gh|gh|gh|gh|github-cli|gh|-
3|Git|git|git|git|git|git|git|-
3|fnm (Node.js)|fnm|fnm|-|-|fnm|-|fb_fnm
3|uv (Python)|uv|uv|-|-|uv|-|fb_uv
3|Neovim|nvim|neovim|neovim|neovim|neovim|neovim|-
3|Antigravity IDE|antigravity-ide antigravity|cask:antigravity-ide|-|-|-|-|fb_agide
3|Docker|docker|cask:docker-desktop|-|-|-|-|fb_docker
4|Claude Code|claude|-|-|-|-|-|fb_claude
4|Codex CLI|codex|cask:codex|-|-|-|-|fb_codex
4|Antigravity CLI (agy)|agy|cask:antigravity-cli|-|-|-|-|fb_agy
'
G1='Terminal dep + tien: Starship, Nerd Font, fzf, zoxide, goi y + to mau lenh (zsh)'
G2='Tim kiem / xem file: ripgrep, fd, bat, eza, jq, lazygit, delta, btop, gh'
G3='Lap trinh backend: Git, Node (fnm), Python (uv), Neovim, Antigravity IDE, Docker (hoi rieng)'
G4='Coding agent: Claude Code, Codex CLI, Antigravity CLI (agy)'

# ======================================================================
#  CHON NHOM
# ======================================================================
echo
printf '\033[36m  CAI CONG CU DONG LENH (%s, %s)\033[0m\n' "$OS" "${PM:-khong ro trinh quan ly goi}"
printf '   1. %s\n   2. %s\n   3. %s\n   4. %s\n' "$G1" "$G2" "$G3" "$G4"
PICK=$(ask $'\n  Chon nhom (vd: 1 2 4). Enter = tat ca, 0 = huy: ' WJ_GROUPS)
case "$(printf '%s' "$PICK" | tr -d '[:space:]')" in 0) echo "  Da huy, khong cai gi."; exit 0 ;; esac
SEL=$(printf '%s' "$PICK" | tr -c '1234' ' ')
[ -z "$(printf '%s' "$SEL" | tr -d ' ')" ] && SEL="1 2 3 4"
ok "  -> Cai nhom: $(echo $SEL)"

install_line() {
    local g="$1" name="$2" chk="$3" pb="$4" pa="$5" pd="$6" pp="$7" pz="$8" fb="$9" pkg=""
    if is_ok "$chk"; then note "$name" "da co"; return; fi
    if [ "$name" = "Docker" ]; then
        local a; a=$(ask "   Cai Docker? Nang (~1GB). (y/N) " WJ_DOCKER)
        case "$a" in y|Y|yes) ;; *) note "$name" "bo qua"; return ;; esac
    fi
    printf '   Cai %s ...' "$name"
    case "$PM" in brew) pkg="$pb" ;; apt) pkg="$pa" ;; dnf) pkg="$pd" ;; pacman) pkg="$pp" ;; zypper) pkg="$pz" ;; esac
    local done_ok=1
    if [ -n "$pkg" ] && [ "$pkg" != "-" ]; then pm_install "$pkg" && done_ok=0; fi
    hash -r 2>/dev/null
    local rc=0
    if ! is_ok "$chk" && [ "$fb" != "-" ]; then "$fb"; rc=$?; [ "$rc" = 0 ] && done_ok=0; fi
    hash -r 2>/dev/null
    if is_ok "$chk"; then ok " OK"; note "$name" "OK"
    elif [ "$done_ok" = 0 ]; then ok " OK (mo terminal moi de dung)"; note "$name" "OK, mo terminal moi"
    elif [ "$rc" = 3 ]; then warn " tu tai"; note "$name" "tu tai: $MANUAL"
    else printf '\033[31m LOI\033[0m\n'; note "$name" "loi"
    fi
}

pm_prepare
for g in $SEL; do
    eval "gt=\$G$g"
    say "Nhom $g: ${gt%%:*}"
    while IFS='|' read -r pg name chk pb pa pd pp pz fb; do
        [ "$pg" = "$g" ] || continue
        case " ${DONE:-} " in *" $name "*) continue ;; esac
        install_line "$pg" "$name" "$chk" "$pb" "$pa" "$pd" "$pp" "$pz" "$fb"
    done <<EOF
$PKGS
EOF
done
hash -r 2>/dev/null
# Debian/Ubuntu dat ten khac. Dung link thay vi alias de script va coding agent cung goi duoc.
has fd  || { has fdfind && ln -sf "$(command -v fdfind)" "$BIN/fd"; }
has bat || { has batcat && ln -sf "$(command -v batcat)" "$BIN/bat"; }
hash -r 2>/dev/null

# ======================================================================
#  CAU HINH SHELL
# ======================================================================
read -r -d '' RC_BLOCK <<'RCEOF'
# Tao boi setup.sh. Chay lai se ghi de khoi nay; cau hinh rieng viet ngoai khoi.
if [ -n "${ZSH_VERSION:-}" ]; then _wj_sh=zsh; else _wj_sh=bash; fi
[ -x /opt/homebrew/bin/brew ] && eval "$(/opt/homebrew/bin/brew shellenv)"
[ -x /home/linuxbrew/.linuxbrew/bin/brew ] && eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"
case ":$PATH:" in *":$HOME/.local/bin:"*) ;; *) export PATH="$HOME/.local/bin:$PATH" ;; esac
[ -d "$HOME/.local/share/fnm" ] && export PATH="$HOME/.local/share/fnm:$PATH"
_wj_has() { command -v "$1" >/dev/null 2>&1; }

_wj_has fnm      && eval "$(fnm env --use-on-cd --shell $_wj_sh)"
_wj_has starship && eval "$(starship init $_wj_sh)"
_wj_has zoxide   && eval "$(zoxide init $_wj_sh)"
if _wj_has fzf; then   # Ctrl+R tim lenh cu, Ctrl+T tim file
    if fzf --$_wj_sh >/dev/null 2>&1; then eval "$(fzf --$_wj_sh)"
    else for _f in "$HOME/.local/share/wisjoy-fzf/key-bindings.$_wj_sh" /usr/share/doc/fzf/examples/key-bindings.$_wj_sh /usr/share/fzf/key-bindings.$_wj_sh /usr/share/fzf/shell/key-bindings.$_wj_sh /opt/homebrew/opt/fzf/shell/key-bindings.$_wj_sh; do
        [ -f "$_f" ] && . "$_f" && break; done
    fi
fi
if [ "$_wj_sh" = zsh ]; then   # goi y lenh mo (mui ten phai de nhan) + to mau lenh
    for _p in zsh-autosuggestions zsh-syntax-highlighting; do
        for _d in /opt/homebrew/share /usr/local/share /usr/share /usr/share/zsh/plugins; do
            [ -f "$_d/$_p/$_p.zsh" ] && . "$_d/$_p/$_p.zsh" && break
        done
    done
fi

# ---- Xem file ----
_wj_has batcat && ! _wj_has bat && alias bat='batcat'
_wj_has fdfind && ! _wj_has fd && alias fd='fdfind'
if _wj_has eza; then
    alias ls='eza --icons --group-directories-first'
    alias ll='eza -la --icons --git --group-directories-first'
    alias lt='eza --tree --level=2 --icons --git-ignore'
else
    alias ll='ls -lah'
fi
alias ..='cd ..'
alias ...='cd ../..'
mkcd() { mkdir -p "$1" && cd "$1"; }
reload() { . "$HOME/.${_wj_sh}rc"; }

# ---- Git ----
alias g='git'
alias gs='git status -sb'
alias gd='git diff'
alias glog='git log --oneline --graph --decorate -20'
alias lg='lazygit'

# ---- Coding agent ----
alias cl='claude'
alias clr='claude --resume'
alias cx='codex'

tips() {
    cat <<'TIPS'

  XEM FILE       ls / ll / lt      danh sach, chi tiet + git, dang cay
                 bat file          xem file co to mau
  DI CHUYEN      z ten             nhay toi thu muc hay vao (vd: z proj)
                 zi                chon thu muc bang danh sach
                 .. / ... / mkcd   len 1 cap / 2 cap / tao va vao thu muc
  TIM KIEM       rg chu            tim chu trong toan bo code
                 fd ten            tim file theo ten
                 Ctrl+R / Ctrl+T   tim lenh cu / tim file (fzf)
                 mui ten phai      nhan goi y lenh mo (zsh)
  GIT            g / gs / gd       git / status gon / diff
                 glog / lg         lich su dang cay / lazygit
  AGENT          cl / clr          claude / mo lai phien cu
                 cx / agy          codex / antigravity
  KHAC           btop / jq / reload

TIPS
}
RCEOF

set_block() { # set_block file
    local f="$1" tmp
    [ -f "$f" ] && cp "$f" "$f.bak-wisjoy"
    touch "$f"
    tmp=$(mktemp)
    awk -v a="$MARK_A" -v b="$MARK_B" '$0==a{skip=1;next} $0==b{skip=0;next} !skip' "$f" > "$tmp"
    # bo dong trong thua o cuoi
    printf '%s\n' "$(cat "$tmp")" > "$f"
    [ -s "$f" ] && [ "$(tr -d '[:space:]' < "$f")" != "" ] && printf '\n' >> "$f"
    printf '%s\n%s\n%s\n' "$MARK_A" "$RC_BLOCK" "$MARK_B" >> "$f"
    rm -f "$tmp"
}

say "Cau hinh shell"
RCS=""
if [ "$OS" = "Darwin" ]; then RCS="$HOME/.zshrc"; [ -f "$HOME/.bashrc" ] && RCS="$RCS $HOME/.bashrc"
else RCS="$HOME/.bashrc"; { has zsh || [ -f "$HOME/.zshrc" ]; } && RCS="$RCS $HOME/.zshrc"
fi
for f in $RCS; do set_block "$f"; echo "   $f"; done
# macOS: bash dang nhap doc ~/.bash_profile, dam bao no goi ~/.bashrc
if [ "$OS" = "Darwin" ] && [ -f "$HOME/.bashrc" ] && ! grep -q '.bashrc' "$HOME/.bash_profile" 2>/dev/null; then
    printf '\n[ -f ~/.bashrc ] && . ~/.bashrc\n' >> "$HOME/.bash_profile"
fi

# fzf ban cu (Ubuntu/Debian) khong co 'fzf --bash': tai file phim tat dung phien ban
if has fzf && ! fzf --bash </dev/null >/dev/null 2>&1; then
    fv=$(fzf --version 2>/dev/null | awk '{print $1}')
    fd_="$HOME/.local/share/wisjoy-fzf"; mkdir -p "$fd_"
    for sh_ in bash zsh; do
        [ -f "/usr/share/doc/fzf/examples/key-bindings.$sh_" ] && continue
        for t in "$fv" "v$fv"; do
            curl -fsSL "https://raw.githubusercontent.com/junegunn/fzf/$t/shell/key-bindings.$sh_" -o "$fd_/key-bindings.$sh_" 2>/dev/null && break
            rm -f "$fd_/key-bindings.$sh_"
        done
    done
    echo "   fzf: phim tat Ctrl+R / Ctrl+T cho ban $fv"
fi

# Starship: cho quet thu muc lau hon (thu muc nhieu file -> canh bao scan_timeout)
if has starship && [ ! -f "$HOME/.config/starship.toml" ]; then
    mkdir -p "$HOME/.config"
    printf '# Tao boi setup.sh\nscan_timeout = 200\ncommand_timeout = 1000\n' > "$HOME/.config/starship.toml"
    echo "   Starship: ~/.config/starship.toml"
fi

# Git
if has git; then
    git config --global init.defaultBranch main
    if has delta; then
        git config --global core.pager delta
        git config --global interactive.diffFilter 'delta --color-only'
        git config --global delta.navigate true
        git config --global merge.conflictstyle zdiff3
    fi
    [ -z "$(git config --global user.name)" ] && { n=$(ask '   Ten hien thi cho git (Enter = bo qua): ' WJ_GIT_NAME); [ -n "$n" ] && git config --global user.name "$n"; }
    [ -z "$(git config --global user.email)" ] && { e=$(ask '   Email cho git (Enter = bo qua): ' WJ_GIT_EMAIL); [ -n "$e" ] && git config --global user.email "$e"; }
    echo "   Git: nhanh mac dinh main$(has delta && echo ' + diff bang delta')"
fi

# Node LTS qua fnm
if has fnm; then
    if ! fnm list 2>/dev/null | grep -q 'v[0-9]'; then echo "   Node.js: cai ban LTS..."; fnm install --lts >/dev/null 2>&1; fi
    fnm default lts-latest >/dev/null 2>&1
    echo "   Node.js: $(fnm list 2>/dev/null | grep -o 'v[0-9][0-9.]*' | head -1) (fnm)"
fi

# ======================================================================
#  KET QUA
# ======================================================================
say "Ket qua"
printf '%s' "$RESULTS"
echo
ok "  Mo cua so terminal moi (hoac go: exec \$SHELL), roi go  tips  de xem cac lenh tat."
echo "  Nho doi font terminal sang 'JetBrainsMono Nerd Font' de hien icon."
echo "  Lan dau chay: claude / codex / agy se mo trinh duyet de dang nhap."
case "$RESULTS" in *" loi"*) warn "  Co phan cai loi: chay lai wj tools install cli."; exit 1 ;; esac
case " $SEL " in *" 3 "*) [ "$OS" = "Linux" ] && has docker && [ "$(id -u)" -ne 0 ] && echo "  Docker: dang xuat / dang nhap lai de dung docker khong can sudo." ;; esac
exit 0
