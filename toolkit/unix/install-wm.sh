#!/usr/bin/env bash
# ======================================================================
#  CUA SO KIEU i3 cho macOS (AeroSpace) va Linux (i3, Hyprland)
#  Cung bo phim voi Komorebi tren Windows:
#   - Alt+h/j/k/l focus, Alt+Shift+h/j/k/l di chuyen, Alt+Ctrl+h/j/k/l resize
#   - Alt+1..0 workspace 1-10 chia cho cac man tu trai sang phai
#   - Alt+Shift+Esc: tat/bat phim tat de remote (Parsec...)
#  Chay lai sau khi cam/rut/doi vi tri man de chia lai workspace.
#  Chay:  bash setup-wm.sh          Go: bash uninstall-wm.sh
# ======================================================================
set -u
set -o pipefail
OS=$(uname -s)
BIN="$HOME/.local/bin"
CFG="${XDG_CONFIG_HOME:-$HOME/.config}"
mkdir -p "$BIN"
export PATH="$BIN:$PATH"
[ -x /opt/homebrew/bin/brew ] && eval "$(/opt/homebrew/bin/brew shellenv)"
[ -x /usr/local/bin/brew ] && eval "$(/usr/local/bin/brew shellenv)"
TOTAL=10
STAMP="Tao boi setup-wm.sh. Chay lai file do se ghi de file nay."

say()  { printf '\n\033[36m>> %s\033[0m\n' "$*"; }
ok()   { printf '\033[32m%s\033[0m\n' "$*"; }
warn() { printf '\033[33m%s\033[0m\n' "$*"; }
has()  { command -v "$1" >/dev/null 2>&1; }
ask()  {
    local v="${2:-}" a=""
    if [ -n "$v" ] && [ -n "${!v:-}" ]; then printf '%s' "${!v}"; return; fi
    if [ -r /dev/tty ] && { true </dev/tty; } 2>/dev/null; then read -r -p "$1" a </dev/tty || a=""; fi
    printf '%s' "$a"
}
# .bak-wisjoy = ban goc cua ban (truoc lan chay dau, khong phai file script tao); .bak-wisjoy.last = ban ngay truoc lan chay nay
backup() {
    [ -e "$1" ] || return 0
    if [ ! -e "$1.bak-wisjoy" ] && ! grep -q "setup-wm.sh" "$1" 2>/dev/null; then cp -R "$1" "$1.bak-wisjoy"; fi
    cp -R "$1" "$1.bak-wisjoy.last"
}

# Chia workspace 1..10 cho N man: moi man floor(10/N), man cuoi lay phan con lai
# In ra: "so_thu_tu_man workspace" moi dong
split_ws() {
    local n="$1" base i next=1 r cnt
    [ "$n" -lt 1 ] && n=1
    base=$(( TOTAL / n ))
    for (( r=1; r<=n; r++ )); do
        if [ "$r" -eq "$n" ]; then cnt=$(( TOTAL - next + 1 )); else cnt=$base; fi
        for (( i=0; i<cnt; i++ )); do echo "$r $next"; next=$((next+1)); done
    done
}

# ======================================================================
#  macOS: AeroSpace
# ======================================================================
mac_terminal_cmd() {
    if   [ -d /Applications/Ghostty.app ];   then echo "open -na Ghostty"
    elif [ -d /Applications/iTerm.app ];     then echo "open -na iTerm"
    elif [ -d /Applications/WezTerm.app ];   then echo "open -na WezTerm"
    elif [ -d /Applications/kitty.app ];     then echo "open -na kitty"
    elif [ -d /Applications/Alacritty.app ]; then echo "open -na Alacritty"
    else echo "osascript -e 'tell application \"Terminal\" to do script \"\"' -e 'tell application \"Terminal\" to activate'"
    fi
}
mac_monitor_count() {
    local n
    n=$(system_profiler SPDisplaysDataType 2>/dev/null | grep -c 'Resolution:')
    [ "${n:-0}" -lt 1 ] && n=1
    echo "$n"
}

write_aerospace() {
    local f="$HOME/.aerospace.toml" term n
    term=$(mac_terminal_cmd)
    n=$(mac_monitor_count)
    backup "$f"
    if [ -f "$CFG/aerospace/aerospace.toml" ]; then
        backup "$CFG/aerospace/aerospace.toml"; rm -f "$CFG/aerospace/aerospace.toml"
        warn "   Da chuyen $CFG/aerospace/aerospace.toml sang ban sao (.bak-wisjoy) de khong trung voi ~/.aerospace.toml"
    fi
    {
        echo "# $STAMP"
        echo "# Muon sua: sua file nay roi Alt+Shift+C. Chay lai setup-wm.sh se ghi de (ban cu: .bak-wisjoy.last)."
        cat <<'EOF'
config-version = 2
start-at-login = true
after-startup-command = []
enable-normalization-flatten-containers = true
enable-normalization-opposite-orientation-for-nested-containers = true
accordion-padding = 20
default-root-container-layout = 'tiles'
# Man ngang xep cot, man doc tu xep hang
default-root-container-orientation = 'auto'
# Chuot nhay theo cua so/man vua focus (giong Komorebi)
on-focused-monitor-changed = ['move-mouse monitor-lazy-center']
on-focus-changed = ['move-mouse window-lazy-center']
automatically-unhide-macos-hidden-apps = false
persistent-workspaces = ["1", "2", "3", "4", "5", "6", "7", "8", "9", "10"]
key-mapping.preset = 'qwerty'

# Khoang cach 1px giong Komorebi
gaps.inner.horizontal = 1
gaps.inner.vertical = 1
gaps.outer.left = 1
gaps.outer.bottom = 1
gaps.outer.top = 1
gaps.outer.right = 1

[mode.main.binding]
EOF
        echo "    alt-enter = '''exec-and-forget $term'''"
        if [ -d /Applications/Raycast.app ]; then
            echo "    # alt-d: dat phim Alt+D trong Raycast (Settings > General > Raycast Hotkey)"
        else
            echo "    alt-d = '''exec-and-forget osascript -e 'tell application \"System Events\" to keystroke space using command down' '''   # mo Spotlight"
        fi
        cat <<'EOF'
    alt-q = 'close'

    alt-h = 'focus left'
    alt-j = 'focus down'
    alt-k = 'focus up'
    alt-l = 'focus right'
    alt-left = 'focus left'
    alt-down = 'focus down'
    alt-up = 'focus up'
    alt-right = 'focus right'

    alt-shift-h = 'move left'
    alt-shift-j = 'move down'
    alt-shift-k = 'move up'
    alt-shift-l = 'move right'
    alt-shift-left = 'move left'
    alt-shift-down = 'move down'
    alt-shift-up = 'move up'
    alt-shift-right = 'move right'

    # Alt+C xep cot, Alt+V xep hang, Alt+S/W gop thanh chong (accordion), Alt+E tach lai
    alt-c = 'layout h_tiles'
    alt-v = 'layout v_tiles'
    alt-s = 'layout v_accordion'
    alt-w = 'layout h_accordion'
    alt-e = 'layout tiles'

    alt-f = 'fullscreen'
    alt-shift-space = 'layout floating tiling'
    alt-shift-minus = 'macos-native-minimize'

    # Resize: h hep, j cao, k thap, l rong (Alt+R vao che do resize giong i3)
    alt-ctrl-h = 'resize width -50'
    alt-ctrl-j = 'resize height +50'
    alt-ctrl-k = 'resize height -50'
    alt-ctrl-l = 'resize width +50'
    alt-r = 'mode resize'

    # Man hinh
    alt-comma = 'focus-monitor --wrap-around prev'
    alt-period = 'focus-monitor --wrap-around next'
    alt-shift-comma = 'move-node-to-monitor --wrap-around --focus-follows-window prev'
    alt-shift-period = 'move-node-to-monitor --wrap-around --focus-follows-window next'

    alt-shift-w = 'balance-sizes'
    alt-shift-c = 'reload-config'
    # Bat/tat Dock (giong Alt+B an taskbar)
    alt-b = '''exec-and-forget osascript -e 'tell application "System Events" to set autohide of dock preferences to not (autohide of dock preferences)' '''

    # Remote (Parsec...): tat phim tat may nay, bam lai de bat
    alt-shift-esc = 'mode remote'

    # Workspace 1..10 (nhay thang du o man nao)
EOF
        local i k
        for (( i=1; i<=TOTAL; i++ )); do k=$(( i % 10 )); echo "    alt-$k = 'workspace $i'"; done
        for (( i=1; i<=TOTAL; i++ )); do k=$(( i % 10 )); echo "    alt-shift-$k = 'move-node-to-workspace $i'"; done
        cat <<'EOF'

[mode.resize.binding]
    h = 'resize width -50'
    j = 'resize height +50'
    k = 'resize height -50'
    l = 'resize width +50'
    left = 'resize width -50'
    down = 'resize height +50'
    up = 'resize height -50'
    right = 'resize width +50'
    esc = 'mode main'
    enter = 'mode main'
    alt-r = 'mode main'

[mode.remote.binding]
    alt-shift-esc = 'mode main'

# Workspace chia cho cac man tu trai sang phai (1 = man trai nhat)
[workspace-to-monitor-force-assignment]
EOF
        split_ws "$n" | while read -r mon ws; do echo "    '$ws' = $mon"; done
    } > "$f"
    echo "   ~/.aerospace.toml ($n man: $(split_ws "$n" | awk '{a[$1]=a[$1] (a[$1]?",":"") $2} END{for(i=1;i in a;i++) printf "%s%s", (i>1?" | ":""), a[i]}'))"
}

setup_mac() {
    say "macOS: AeroSpace"
    if [ ! -d /Applications/AeroSpace.app ] && ! has aerospace; then
        if ! has brew; then
            say "Cai Homebrew (se hoi mat khau may Mac)"
            /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
            [ -x /opt/homebrew/bin/brew ] && eval "$(/opt/homebrew/bin/brew shellenv)"
            [ -x /usr/local/bin/brew ] && eval "$(/usr/local/bin/brew shellenv)"
        fi
        printf '   Cai AeroSpace ...'
        if brew install --cask nikitabobko/tap/aerospace >/dev/null 2>&1; then ok " OK"; else printf '\033[31m LOI\033[0m\n'; fi
    else
        echo "   AeroSpace: da co"
    fi
    write_aerospace

    say "Cai dat macOS cho AeroSpace chay muot"
    # AeroSpace khuyen: tat "Displays have separate Spaces", bat "Group windows by application"
    defaults write com.apple.spaces spans-displays -bool true
    defaults write com.apple.dock expose-group-apps -bool true
    defaults write com.apple.dock autohide -bool true
    killall Dock 2>/dev/null
    echo "   Tat 'Displays have separate Spaces' (can dang xuat 1 lan), an Dock (Alt+B bat lai)"


    if [ -d /Applications/AeroSpace.app ]; then
        open -a AeroSpace
        sleep 2
        has aerospace && aerospace reload-config >/dev/null 2>&1
    fi
}

# ======================================================================
#  Linux: i3 va Hyprland
# ======================================================================
lin_terminal() {
    local t
    for t in ghostty kitty alacritty wezterm foot gnome-terminal konsole xfce4-terminal; do has "$t" && { echo "$t"; return; }; done
    if [ "${1:-}" = x ]; then echo "i3-sensible-terminal"; else echo "kitty"; fi
}
# Ten man cua X (i3), sap trai -> phai
x_outputs() {
    has xrandr || return 0
    xrandr --listmonitors 2>/dev/null | awk 'NR>1 { n=$NF; split($3, g, "+"); print g[2], n }' | sort -n | awk '{print $2}'
}
# Ten man cua Hyprland, sap trai -> phai
hypr_outputs() {
    has hyprctl || return 0
    [ -n "${HYPRLAND_INSTANCE_SIGNATURE:-}" ] || return 0
    hyprctl monitors 2>/dev/null | awk '/^Monitor /{n=$2} / at -?[0-9]+x-?[0-9]+/{for(i=1;i<=NF;i++) if($i=="at"){split($(i+1),p,"x"); print p[1], n}}' | sort -n | awk '{print $2}'
}

write_i3() {
    local d="$CFG/i3" f term outs n i mon
    f="$d/config"
    mkdir -p "$d"
    backup "$f"
    [ -f "$d/rieng.conf" ] || printf '# Cau hinh rieng cua ban cho i3. setup-wm.sh KHONG ghi de file nay.\n' > "$d/rieng.conf"
    term=$(lin_terminal x)
    outs=$(x_outputs)
    n=$(printf '%s\n' "$outs" | grep -c .)
    {
        echo "# $STAMP"
        echo "# Cau hinh rieng viet vao ~/.config/i3/rieng.conf (duoc nap o cuoi file)."
        cat <<EOF
set \$mod Mod1
set \$term $term
set \$menu rofi -show drun || dmenu_run
EOF
        cat <<'EOF'
font pango:JetBrainsMono Nerd Font 10
default_border pixel 2
default_floating_border pixel 2
gaps inner 1
gaps outer 0
default_orientation auto
focus_follows_mouse no
mouse_warping output
floating_modifier $mod

# ---- Mo ung dung ----
bindsym $mod+Return exec $term
bindsym $mod+d exec --no-startup-id sh -c '$menu'
bindsym $mod+q kill

# ---- Focus / di chuyen: h j k l (kieu vim) va mui ten ----
bindsym $mod+h focus left
bindsym $mod+j focus down
bindsym $mod+k focus up
bindsym $mod+l focus right
bindsym $mod+Left focus left
bindsym $mod+Down focus down
bindsym $mod+Up focus up
bindsym $mod+Right focus right
bindsym $mod+Shift+h move left
bindsym $mod+Shift+j move down
bindsym $mod+Shift+k move up
bindsym $mod+Shift+l move right
bindsym $mod+Shift+Left move left
bindsym $mod+Shift+Down move down
bindsym $mod+Shift+Up move up
bindsym $mod+Shift+Right move right

# ---- Chia / layout ----
bindsym $mod+c split h
bindsym $mod+v split v
bindsym $mod+s layout stacking
bindsym $mod+w layout tabbed
bindsym $mod+e layout toggle split
bindsym $mod+f fullscreen toggle
bindsym $mod+Shift+space floating toggle
bindsym $mod+space focus mode_toggle
bindsym $mod+a focus parent
bindsym $mod+Shift+minus move scratchpad
bindsym $mod+minus scratchpad show

# ---- Resize: h hep, j cao, k thap, l rong ----
bindsym $mod+Ctrl+h resize shrink width 10 px or 5 ppt
bindsym $mod+Ctrl+j resize grow height 10 px or 5 ppt
bindsym $mod+Ctrl+k resize shrink height 10 px or 5 ppt
bindsym $mod+Ctrl+l resize grow width 10 px or 5 ppt
mode "resize" {
    bindsym h resize shrink width 10 px or 5 ppt
    bindsym j resize grow height 10 px or 5 ppt
    bindsym k resize shrink height 10 px or 5 ppt
    bindsym l resize grow width 10 px or 5 ppt
    bindsym Return mode "default"
    bindsym Escape mode "default"
    bindsym $mod+r mode "default"
}
bindsym $mod+r mode "resize"

# ---- Man hinh ----
bindsym $mod+comma focus output left
bindsym $mod+period focus output right
bindsym $mod+Shift+comma move container to output left; focus output left
bindsym $mod+Shift+period move container to output right; focus output right
bindsym $mod+Ctrl+Shift+h move workspace to output left
bindsym $mod+Ctrl+Shift+l move workspace to output right

# ---- Remote (Parsec...): tat phim tat may nay, bam lai de bat ----
mode "remote" {
    bindsym $mod+Shift+Escape mode "default"
}
bindsym $mod+Shift+Escape mode "remote"

# ---- Quan ly ----
bindsym $mod+b bar mode toggle
bindsym $mod+Shift+c reload
bindsym $mod+Shift+r restart
bindsym $mod+Shift+e exec "i3-nagbar -t warning -m 'Thoat i3?' -B 'Thoat' 'i3-msg exit'"

# ---- Workspace 1..10 ----
EOF
        for (( i=1; i<=TOTAL; i++ )); do echo "bindsym \$mod+$(( i % 10 )) workspace number $i"; done
        for (( i=1; i<=TOTAL; i++ )); do echo "bindsym \$mod+Shift+$(( i % 10 )) move container to workspace number $i"; done
        if [ "$n" -gt 0 ]; then
            echo ""
            echo "# Workspace chia cho cac man tu trai sang phai (luc chay script)"
            split_ws "$n" | while read -r r ws; do
                mon=$(printf '%s\n' "$outs" | sed -n "${r}p")
                echo "workspace $ws output $mon primary"
            done
        fi
        cat <<'EOF'

bar {
    position top
    status_command i3status
    font pango:JetBrainsMono Nerd Font 10
}

include ~/.config/i3/rieng.conf
EOF
    } > "$f"
    echo "   $f ($n man)"
    if has i3; then
        if i3 -C -c "$f" >/dev/null 2>&1; then
            [ -n "${DISPLAY:-}" ] && i3-msg reload >/dev/null 2>&1
            ok "   i3: cau hinh hop le$( [ -n "${DISPLAY:-}" ] && echo ', da nap lai')"
        else
            warn "   i3 bao loi cau hinh:"; i3 -C -c "$f" 2>&1 | tail -5
        fi
    fi
}

write_hypr_lua() {
    local d="$1" f term outs n i mon
    f="$d/hyprland.lua"
    term=$(lin_terminal)
    outs=$(hypr_outputs)
    n=$(printf '%s\n' "$outs" | grep -c .)
    backup "$f"
    [ -f "$d/rieng.lua" ] || printf -- '-- Cau hinh rieng cua ban cho Hyprland. setup-wm.sh KHONG ghi de file nay.\n' > "$d/rieng.lua"
    {
        echo "-- $STAMP"
        echo "-- Cau hinh rieng viet vao ~/.config/hypr/rieng.lua (duoc nap o cuoi file)."
        echo "local terminal = \"$term\""
        cat <<'EOF'
local menu     = "rofi -show drun || wofi --show drun || hyprlauncher"
local mod      = "ALT"

hl.monitor({ output = "", mode = "preferred", position = "auto", scale = "auto" })

hl.config({
    general    = { gaps_in = 1, gaps_out = 1, border_size = 2, layout = "dwindle" },
    decoration = { rounding = 0 },
    dwindle    = { preserve_split = true },
    input      = { kb_layout = "us", follow_mouse = 1 },
    misc       = { force_default_wallpaper = 0, disable_hyprland_logo = true },
})

hl.on("hyprland.start", function()
    hl.exec_cmd("command -v waybar >/dev/null && waybar")
end)

local function key(k) return mod .. " + " .. k end

-- ---- Mo ung dung ----
hl.bind(key("Return"), hl.dsp.exec_cmd(terminal))
hl.bind(key("D"),      hl.dsp.exec_cmd(menu))
hl.bind(key("Q"),      hl.dsp.window.close())

-- ---- Focus / di chuyen / resize: h j k l (kieu vim) va mui ten ----
local dirs = { H = "left", J = "down", K = "up", L = "right", left = "left", down = "down", up = "up", right = "right" }
local grow = { left = { -40, 0 }, down = { 0, 40 }, up = { 0, -40 }, right = { 40, 0 } }
for k, d in pairs(dirs) do
    hl.bind(key(k),              hl.dsp.focus({ direction = d }))
    hl.bind(key("SHIFT + " .. k), hl.dsp.window.move({ direction = d }))
    hl.bind(key("CTRL + " .. k),  hl.dsp.window.resize({ x = grow[d][1], y = grow[d][2], relative = true }), { repeating = true })
end

-- ---- Chia / layout ----
hl.bind(key("C"), hl.dsp.layout("preselect r"))     -- cua so tiep theo mo ben phai (cot)
hl.bind(key("V"), hl.dsp.layout("preselect d"))     -- cua so tiep theo mo ben duoi (hang)
hl.bind(key("E"), hl.dsp.layout("togglesplit"))
hl.bind(key("S"), hl.dsp.group.toggle())            -- gop thanh nhom (tab)
hl.bind(key("W"), hl.dsp.group.toggle())
hl.bind(key("bracketleft"),  hl.dsp.group.prev())
hl.bind(key("bracketright"), hl.dsp.group.next())
hl.bind(key("F"), hl.dsp.window.fullscreen())
hl.bind(key("SHIFT + space"), hl.dsp.window.float({ action = "toggle" }))
hl.bind(key("SHIFT + minus"), hl.dsp.window.move({ workspace = "special:scratch" }))
hl.bind(key("minus"),         hl.dsp.workspace.toggle_special("scratch"))
hl.bind(key("mouse:272"), hl.dsp.window.drag(),   { mouse = true })
hl.bind(key("mouse:273"), hl.dsp.window.resize(), { mouse = true })

-- ---- Man hinh ----
hl.bind(key("comma"),          hl.dsp.focus({ monitor = "-1" }))
hl.bind(key("period"),         hl.dsp.focus({ monitor = "+1" }))
hl.bind(key("SHIFT + comma"),  hl.dsp.window.move({ monitor = "-1" }))
hl.bind(key("SHIFT + period"), hl.dsp.window.move({ monitor = "+1" }))
hl.bind(key("CTRL + SHIFT + H"), hl.dsp.workspace.move({ monitor = "-1" }))
hl.bind(key("CTRL + SHIFT + L"), hl.dsp.workspace.move({ monitor = "+1" }))

-- ---- Workspace 1..10 ----
for i = 1, 10 do
    local k = tostring(i % 10)
    hl.bind(key(k),              hl.dsp.focus({ workspace = i }))
    hl.bind(key("SHIFT + " .. k), hl.dsp.window.move({ workspace = i }))
end

-- ---- Remote (Parsec...): tat phim tat may nay, bam lai de bat ----
hl.bind(key("SHIFT + Escape"), hl.dsp.submap("remote"))
hl.define_submap("remote", function()
    hl.bind(key("SHIFT + Escape"), hl.dsp.submap("reset"))
end)

-- ---- Quan ly ----
hl.bind(key("B"),         hl.dsp.exec_cmd("pkill -SIGUSR1 waybar"))
hl.bind(key("SHIFT + C"), hl.dsp.exec_cmd("hyprctl reload"))
hl.bind(key("SHIFT + E"), hl.dsp.exit())

EOF
        if [ "$n" -gt 0 ]; then
            echo ""
            echo "-- Workspace chia cho cac man tu trai sang phai (luc chay script)"
            split_ws "$n" | while read -r r ws; do
                mon=$(printf '%s\n' "$outs" | sed -n "${r}p")
                echo "hl.workspace_rule({ workspace = \"$ws\", monitor = \"$mon\" })"
            done
        fi
        echo ""
        echo "pcall(require, \"rieng\")"
    } > "$f"
    echo "   $f ($n man)"
}

write_hypr_conf() {   # Hyprland cu (< 0.55) chi doc hyprland.conf
    local d="$1" f term outs n i mon
    f="$d/hyprland.conf"
    term=$(lin_terminal)
    outs=$(hypr_outputs)
    n=$(printf '%s\n' "$outs" | grep -c .)
    backup "$f"
    [ -f "$d/rieng.conf" ] || printf '# Cau hinh rieng cua ban cho Hyprland. setup-wm.sh KHONG ghi de file nay.\n' > "$d/rieng.conf"
    {
        echo "# $STAMP"
        echo "# Cau hinh rieng viet vao ~/.config/hypr/rieng.conf (duoc nap o cuoi file)."
        echo "\$terminal = $term"
        cat <<'EOF'
$menu = rofi -show drun || wofi --show drun
$mod = ALT
monitor = , preferred, auto, auto
exec-once = command -v waybar >/dev/null && waybar

general {
    gaps_in = 1
    gaps_out = 1
    border_size = 2
    layout = dwindle
}
decoration {
    rounding = 0
}
dwindle {
    preserve_split = true
}
input {
    kb_layout = us
    follow_mouse = 1
}
misc {
    force_default_wallpaper = 0
    disable_hyprland_logo = true
}

bind = $mod, Return, exec, $terminal
bind = $mod, D, exec, $menu
bind = $mod, Q, killactive,

bind = $mod, H, movefocus, l
bind = $mod, J, movefocus, d
bind = $mod, K, movefocus, u
bind = $mod, L, movefocus, r
bind = $mod, left, movefocus, l
bind = $mod, down, movefocus, d
bind = $mod, up, movefocus, u
bind = $mod, right, movefocus, r
bind = $mod SHIFT, H, movewindow, l
bind = $mod SHIFT, J, movewindow, d
bind = $mod SHIFT, K, movewindow, u
bind = $mod SHIFT, L, movewindow, r
binde = $mod CTRL, H, resizeactive, -40 0
binde = $mod CTRL, J, resizeactive, 0 40
binde = $mod CTRL, K, resizeactive, 0 -40
binde = $mod CTRL, L, resizeactive, 40 0

bind = $mod, C, layoutmsg, preselect r
bind = $mod, V, layoutmsg, preselect d
bind = $mod, E, togglesplit,
bind = $mod, S, togglegroup,
bind = $mod, W, togglegroup,
bind = $mod, bracketleft, changegroupactive, b
bind = $mod, bracketright, changegroupactive, f
bind = $mod, F, fullscreen, 0
bind = $mod SHIFT, space, togglefloating,
bind = $mod SHIFT, minus, movetoworkspace, special:scratch
bind = $mod, minus, togglespecialworkspace, scratch
bindm = $mod, mouse:272, movewindow
bindm = $mod, mouse:273, resizewindow

bind = $mod, comma, focusmonitor, -1
bind = $mod, period, focusmonitor, +1
bind = $mod SHIFT, comma, movewindow, mon:-1
bind = $mod SHIFT, period, movewindow, mon:+1
bind = $mod CTRL SHIFT, H, movecurrentworkspacetomonitor, -1
bind = $mod CTRL SHIFT, L, movecurrentworkspacetomonitor, +1

bind = $mod SHIFT, Escape, submap, remote
submap = remote
bind = $mod SHIFT, Escape, submap, reset
submap = reset

bind = $mod, B, exec, pkill -SIGUSR1 waybar
bind = $mod SHIFT, C, exec, hyprctl reload
bind = $mod SHIFT, E, exit,

EOF
        for (( i=1; i<=TOTAL; i++ )); do echo "bind = \$mod, $(( i % 10 )), workspace, $i"; done
        for (( i=1; i<=TOTAL; i++ )); do echo "bind = \$mod SHIFT, $(( i % 10 )), movetoworkspace, $i"; done
        if [ "$n" -gt 0 ]; then
            echo ""
            split_ws "$n" | while read -r r ws; do
                mon=$(printf '%s\n' "$outs" | sed -n "${r}p")
                echo "workspace = $ws, monitor:$mon"
            done
        fi
        echo ""
        echo "source = ~/.config/hypr/rieng.conf"
    } > "$f"
    echo "   $f ($n man)"
}

hypr_version() { Hyprland --version 2>/dev/null | grep -o -m1 '[0-9]\+\.[0-9]\+\.[0-9]\+' | head -1; }

write_hypr() {
    local d="$CFG/hypr" v maj min
    mkdir -p "$d"
    v=$(hypr_version)
    maj=${v%%.*}; min=${v#*.}; min=${min%%.*}
    if [ -z "$v" ] || [ "${maj:-0}" -gt 0 ] || [ "${min:-0}" -ge 55 ]; then
        # Hyprland 0.55+ uu tien hyprland.lua; ban moi chi con doc Lua
        if [ -f "$d/hyprland.conf" ] && ! grep -q "setup-wm.sh" "$d/hyprland.conf"; then
            warn "   hyprland.conf cu cua ban se khong duoc doc nua (Hyprland uu tien hyprland.lua). Van giu nguyen file do."
        fi
        write_hypr_lua "$d"
    else
        write_hypr_conf "$d"
    fi
    if [ -n "${HYPRLAND_INSTANCE_SIGNATURE:-}" ] && has hyprctl; then
        hyprctl reload >/dev/null 2>&1 && ok "   Hyprland: da nap lai"
        hyprctl configerrors 2>/dev/null | grep -v -i "^$\|no errors" | head -5
    fi
}

setup_linux() {
    local has_i3=0 has_hypr=0 pick
    has i3 && has_i3=1
    { has Hyprland || has hyprctl; } && has_hypr=1
    echo "   Tim thay: $([ $has_i3 = 1 ] && echo 'i3 ')$([ $has_hypr = 1 ] && echo 'Hyprland')$([ $has_i3$has_hypr = 00 ] && echo 'chua co i3 / Hyprland')"
    echo "   1. i3   2. Hyprland   (vd: 1 2)"
    pick=$(ask "   Cau hinh cho cai nao? Enter = cai dang co, 0 = huy: " WJ_WM)
    case "$(printf '%s' "$pick" | tr -d '[:space:]')" in 0) echo "   Da huy, khong cau hinh gi."; return 0 ;; esac
    if [ -z "$(printf '%s' "$pick" | tr -dc '12')" ]; then
        pick=""; [ $has_i3 = 1 ] && pick="1"; [ $has_hypr = 1 ] && pick="$pick 2"
        [ -z "$pick" ] && pick="1 2"
    fi
    case " $(printf '%s' "$pick" | tr -c '12' ' ') " in *" 1 "*|*1*) say "i3"; write_i3 ;; esac
    case "$pick" in *2*) say "Hyprland"; write_hypr ;; esac
    has rofi || warn "   Alt+D dung rofi neu co (dang dung dmenu/wofi thay the)"
}

# ======================================================================
echo
printf '\033[36m  CUA SO KIEU i3 (%s)\033[0m\n' "$OS"
case "$OS" in
    Darwin) setup_mac ;;
    Linux)  setup_linux ;;
    *) warn "He dieu hanh chua ho tro: $OS"; exit 1 ;;
esac
say "XONG"
ok "  Alt+Enter terminal | Alt+h/j/k/l focus | Alt+1..0 workspace | Alt+Shift+Esc che do remote"
echo "  Cam/rut/doi vi tri man: chay lai file nay de chia lai workspace."
[ "$OS" = "Darwin" ] && echo "  Lan dau: macOS hoi quyen Accessibility cho AeroSpace -> bat trong System Settings > Privacy & Security. Dang xuat 1 lan."
exit 0
