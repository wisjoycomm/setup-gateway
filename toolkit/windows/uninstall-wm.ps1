# ======================================================================
#  GO KOMOREBI: tat + go komorebi, whkd (va GlazeWM/Zebar neu con),
#  xoa config, xoa tu khoi dong, tra taskbar Windows ve binh thuong.
#  Khong dong toi 4 che do nguon.
#  Chay lai bao nhieu lan cung duoc.
# ======================================================================
$ProgressPreference = 'SilentlyContinue'
$utf8 = New-Object Text.UTF8Encoding $false
$HOME_ = $env:USERPROFILE
$extras = Join-Path $HOME_ '.config\komorebi-extras'
$cfgFile = Join-Path $HOME_ 'komorebi.json'
$barTpl = Join-Path $HOME_ 'komorebi.bar.json'
$whkdrc = Join-Path $HOME_ '.config\whkdrc'
$startPs = Join-Path $extras 'komorebi-start.ps1'
$togglePs = Join-Path $extras 'toggle-taskbar.ps1'
$startup = [Environment]::GetFolderPath('Startup')
$programs = [Environment]::GetFolderPath('Programs')
$TOTAL = 10       # so workspace
$BAR_H = 26       # chieu cao thanh bar (px)
$PLANS = [ordered]@{
    '1' = '6c1f0a01-0001-4a00-8a00-00000000a001'
    '2' = '6c1f0a01-0002-4a00-8a00-00000000a002'
    '3' = '6c1f0a01-0003-4a00-8a00-00000000a003'
    '4' = '6c1f0a01-0004-4a00-8a00-00000000a004'
}

function Say($msg, $color = 'Cyan') { Write-Host "`n>> $msg" -ForegroundColor $color }
function Refresh-Path { $env:Path = [Environment]::GetEnvironmentVariable('Path', 'Machine') + ';' + [Environment]::GetEnvironmentVariable('Path', 'User') }
function KC { (Get-Command komorebic -ErrorAction SilentlyContinue).Source }
# winget: tim ca khi PATH thieu (Windows ban Trung / may moi chua dang ky App Installer)
function Find-Winget {
    if ($script:WG) { return $script:WG }
    $c = Get-Command winget.exe -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty Source
    $alias = Join-Path $env:LOCALAPPDATA 'Microsoft\WindowsApps\winget.exe'
    if (-not $c -and (Test-Path $alias)) { $c = $alias }
    if (-not $c) {
        try { Add-AppxPackage -RegisterByFamilyName -MainPackage Microsoft.DesktopAppInstaller_8wekyb3d8bbwe -ErrorAction Stop } catch {}
        Start-Sleep 2
        if (Test-Path $alias) { $c = $alias }
    }
    if (-not $c) {
        $c = Get-ChildItem (Join-Path $env:ProgramFiles 'WindowsApps') -Filter winget.exe -Recurse -ErrorAction SilentlyContinue |
            Where-Object { $_.FullName -like '*DesktopAppInstaller*' } | Sort-Object FullName -Descending | Select-Object -First 1 -ExpandProperty FullName
    }
    $script:WG = $c
    return $c
}
function winget {
    $w = Find-Winget
    if ($w) { & $w @args; return }
    if (-not $script:WGWarned) {
        $script:WGWarned = $true
        Say 'Khong tim thay winget. Mo Microsoft Store, tim "App Installer" -> Cai dat/Cap nhat, roi chay lai file nay.' 'Red'
    }
}
function Set-Prop($obj, $name, $value) {
    if ($obj.PSObject.Properties[$name]) { $obj.$name = $value }
    else { $obj | Add-Member -NotePropertyName $name -NotePropertyValue $value }
}
function Write-Json($obj, $path) { [IO.File]::WriteAllText($path, ($obj | ConvertTo-Json -Depth 30), $utf8) }
function Backup($path) {
    if (Test-Path $path) {
        New-Item -ItemType Directory -Force -Path (Join-Path $extras 'backup') | Out-Null
        Copy-Item $path (Join-Path $extras ('backup\' + (Split-Path $path -Leaf) + '.' + (Get-Date -Format 'yyyyMMdd-HHmmss'))) -Force
    }
}

# ======================================================================
#  PHAN CAN QUYEN ADMIN (chay rieng, Windows se hoi 1 lan)
# ======================================================================
$adminScript = @'
param($mode)
$BAL = '381b4222-f694-41f0-9685-ff5bb260df2e'
$PROC = '54533251-82be-4824-96c1-47b60b740d00'
$PMIN = '893dee8e-2bef-41e0-89c6-b55d0929964c'; $PMAX = 'bc5038f7-23e0-4960-96da-33abaf5935ec'
$COOL = '94d3a615-a899-4ac5-ae2b-e4d8f634367f'; $BOOST = 'be337238-0d82-4146-a960-4f3749d470c7'
$PCIE = '501a4d13-42af-4429-9fd1-a8218c268e20'; $ASPM = 'ee12f906-d277-404b-b6da-e5fa1a576df5'
$USB = '2a737441-1930-4402-8d77-b2bebba308a3';  $USBS = '48e6b7a6-50f5-4782-a5d4-53bb8f07e226'
$VID = '7516b95f-f776-4464-8c53-06167f40cc99';  $VIDL = '3c0bc021-c8a8-4e07-a973-6b14cbcb2b7e'
$SLP = '238c9fa8-0aad-41ed-83f4-97be242c8f20';  $SLPI = '29f6c1db-86da-48c5-9fdb-f2b67b1f44da'
$ids = @('6c1f0a01-0001-4a00-8a00-00000000a001','6c1f0a01-0002-4a00-8a00-00000000a002','6c1f0a01-0003-4a00-8a00-00000000a003','6c1f0a01-0004-4a00-8a00-00000000a004')
function V($g, $sub, $set, $ac, $dc) { powercfg -setacvalueindex $g $sub $set $ac | Out-Null; powercfg -setdcvalueindex $g $sub $set $dc | Out-Null }

if ($mode -eq 'longpaths') {
  Set-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Control\FileSystem' -Name 'LongPathsEnabled' -Value 1 -ErrorAction SilentlyContinue
  exit
}
$active = ((powercfg -getactivescheme) -replace '.*:\s*([0-9a-f-]{36}).*', '$1').Trim()
powercfg -setactive $BAL | Out-Null
foreach ($g in $ids) { powercfg -delete $g 2>$null | Out-Null }

if ($mode -eq 'install') {
  Set-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Control\FileSystem' -Name 'LongPathsEnabled' -Value 1 -ErrorAction SilentlyContinue
  powercfg -attributes $PROC $BOOST -ATTRIB_HIDE | Out-Null
  # ten, mo ta, [xung min] [xung max] [lam mat 0=passive 1=active] [boost 0=tat 2=manh 3=tiet che]
  #            [PCIe 0=tat 1=vua 2=toi da] [USB suspend] [tat man (giay)] [ngu (giay, 0=khong)]
  #  moi cap: cam sac, dung pin
  $defs = @(
    @('1. Tiet kiem pin',  'Di hoc, ngoi cafe, can pin lau',        5,5, 70,50, 0,0, 0,0, 2,2, 1,1, 600,180,  1800,600),
    @('2. Van phong',      'Dung hang ngay: web, van ban, xem phim', 5,5, 80,60, 1,0, 3,0, 1,2, 1,1, 900,300,  1800,900),
    @('3. Choi game',      'CPU 100%, boost tiet che, nho cam sac',  5,5, 100,80, 1,1, 3,3, 0,1, 0,1, 1200,600, 0,1800),
    @('4. Do hoa - Video', 'Render, xuat video, Blender: toi da',    5,5, 100,80, 1,1, 2,3, 0,1, 0,1, 1200,600, 0,1800)
  )
  for ($i = 0; $i -lt 4; $i++) {
    $g = $ids[$i]; $d = $defs[$i]
    powercfg -duplicatescheme $BAL $g | Out-Null
    powercfg -changename $g $d[0] $d[1] | Out-Null
    V $g $PROC $PMIN  $d[2]  $d[3]
    V $g $PROC $PMAX  $d[4]  $d[5]
    V $g $PROC $COOL  $d[6]  $d[7]
    V $g $PROC $BOOST $d[8]  $d[9]
    V $g $PCIE $ASPM  $d[10] $d[11]
    V $g $USB  $USBS  $d[12] $d[13]
    V $g $VID  $VIDL  $d[14] $d[15]
    V $g $SLP  $SLPI  $d[16] $d[17]
  }
  if ($ids -contains $active) { powercfg -setactive $active | Out-Null } else { powercfg -setactive $ids[0] | Out-Null }
}
'@

function Run-Admin($mode) {
    $f = Join-Path $env:TEMP 'setup-workflow-admin.ps1'
    [IO.File]::WriteAllText($f, $adminScript, $utf8)
    try {
        Start-Process powershell -Verb RunAs -Wait -WindowStyle Hidden -ArgumentList "-NoProfile -ExecutionPolicy Bypass -File `"$f`" -mode $mode"
        return $true
    } catch {
        Say 'Ban da bam No o hop hoi quyen admin -> bo qua phan che do nguon.' 'Yellow'
        return $false
    }
}

# ======================================================================
#  CAC SCRIPT PHU (ghi ra %USERPROFILE%\.config\komorebi-extras)
# ======================================================================
$toggleText = @'
$flag = Join-Path $env:USERPROFILE '.config\komorebi-extras\taskbar-show.flag'
if (Test-Path $flag) { Remove-Item $flag -Force } else { New-Item -ItemType File -Path $flag -Force | Out-Null }
'@

$startText = @'
$m = New-Object System.Threading.Mutex($false, 'Local\KomorebiExtras')
if (-not $m.WaitOne(0)) { exit }
$env:Path = [Environment]::GetEnvironmentVariable('Path','Machine') + ';' + [Environment]::GetEnvironmentVariable('Path','User')
$kc = (Get-Command komorebic -ErrorAction SilentlyContinue).Source
if (-not $kc) { exit }

# 1. Bat komorebi + whkd + bar neu chua chay
if (-not (Get-Process -Name komorebi -ErrorAction SilentlyContinue)) {
  & $kc start --whkd --bar | Out-Null
}
for ($i = 0; $i -lt 60 -and -not (Get-Process -Name komorebi -ErrorAction SilentlyContinue); $i++) { Start-Sleep 1 }
Start-Sleep 3

# 2. Man hinh DOC (portrait): moi workspace xep theo hang
function Set-PortraitLayouts {
  Add-Type -AssemblyName System.Windows.Forms
  $screens = [System.Windows.Forms.Screen]::AllScreens
  try { $state = (& $kc state | Out-String) | ConvertFrom-Json } catch { return }
  $mons = @($state.monitors.elements)
  for ($i = 0; $i -lt $mons.Count; $i++) {
    $mon = $mons[$i]; $portrait = $false
    $n = "$($mon.name)" -replace '^\\\\\.\\', ''
    $scr = $null
    if ($n) { $scr = $screens | Where-Object { $_.DeviceName -like "*\$n" } | Select-Object -First 1 }
    if ($scr) { $portrait = $scr.Bounds.Height -gt $scr.Bounds.Width }
    elseif ($mon.size) { $portrait = $mon.size.bottom -gt $mon.size.right }
    if ($portrait) {
      $count = @($mon.workspaces.elements).Count
      for ($w = 0; $w -lt $count; $w++) { & $kc workspace-layout $i $w rows | Out-Null }
    }
  }
}
Set-PortraitLayouts

# 3. An taskbar (Alt+B de bat/tat), tra lai khi komorebi dung
Add-Type @"
using System;
using System.Runtime.InteropServices;
public class TB {
  [StructLayout(LayoutKind.Sequential)] public struct RECT { public int l, t, r, b; }
  [StructLayout(LayoutKind.Sequential)] public struct APPBARDATA {
    public int cbSize; public IntPtr hWnd; public uint uCallbackMessage; public uint uEdge; public RECT rc; public IntPtr lParam;
  }
  [DllImport("shell32.dll")] public static extern UIntPtr SHAppBarMessage(uint msg, ref APPBARDATA d);
  [DllImport("user32.dll")] public static extern IntPtr FindWindow(string c, string w);
  [DllImport("user32.dll")] public static extern IntPtr FindWindowEx(IntPtr p, IntPtr a, string c, string w);
  [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr h, int n);
  [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h);
  public static uint GetState() {
    var d = new APPBARDATA(); d.cbSize = Marshal.SizeOf(d);
    return (uint)SHAppBarMessage(4, ref d);
  }
  public static void SetState(uint s) {
    var d = new APPBARDATA(); d.cbSize = Marshal.SizeOf(d);
    d.hWnd = FindWindow("Shell_TrayWnd", null); d.lParam = (IntPtr)s;
    SHAppBarMessage(10, ref d);
  }
  static void Apply(IntPtr h, int cmd) {
    if (h == IntPtr.Zero) return;
    bool v = IsWindowVisible(h);
    if ((cmd == 0 && v) || (cmd != 0 && !v)) ShowWindow(h, cmd);
  }
  public static void All(int cmd) {
    Apply(FindWindow("Shell_TrayWnd", null), cmd);
    IntPtr s = IntPtr.Zero;
    while ((s = FindWindowEx(IntPtr.Zero, s, "Shell_SecondaryTrayWnd", null)) != IntPtr.Zero) Apply(s, cmd);
  }
}
"@
$orig = [TB]::GetState()
if ($orig -eq 1) { $orig = 0 }   # neu lan truoc bi tat giua chung, tra ve binh thuong
$flag = Join-Path $env:USERPROFILE '.config\komorebi-extras\taskbar-show.flag'
Remove-Item $flag -ErrorAction SilentlyContinue
$shown = $null
$monCount = [System.Windows.Forms.Screen]::AllScreens.Count
try {
  while (Get-Process -Name komorebi -ErrorAction SilentlyContinue) {
    $want = Test-Path $flag
    if ($want -ne $shown) {
      if ($want) { [TB]::SetState(0); [TB]::All(5) } else { [TB]::SetState(1); [TB]::All(0) }
      $shown = $want
      Start-Sleep -Milliseconds 400
      & $kc retile | Out-Null
    }
    if (-not $shown) { [TB]::All(0) }
    # Cam / rut man hinh -> xep lai man doc
    $c = [System.Windows.Forms.Screen]::AllScreens.Count
    if ($c -ne $monCount) { $monCount = $c; Start-Sleep 3; Set-PortraitLayouts }
    Start-Sleep -Milliseconds 400
  }
} finally {
  [TB]::SetState($orig)
  [TB]::All(5)
  $m.ReleaseMutex()
}
'@

# ======================================================================
#  CAC BUOC
# ======================================================================
function Stop-Komorebi {
    $kc = KC
    if ($kc) { & $kc stop --whkd --bar 2>&1 | Out-Null }
    Start-Sleep 1
    Get-Process -Name komorebi, whkd, komorebi-bar -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    Get-CimInstance Win32_Process -Filter "Name='powershell.exe'" -ErrorAction SilentlyContinue |
        Where-Object { $_.CommandLine -like '*komorebi-start.ps1*' } |
        ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
    Start-Sleep 1
}

function Restore-Taskbar {
    if (-not ('TBR' -as [type])) {
        Add-Type @"
using System;
using System.Runtime.InteropServices;
public class TBR {
  [StructLayout(LayoutKind.Sequential)] public struct RECT { public int l, t, r, b; }
  [StructLayout(LayoutKind.Sequential)] public struct APPBARDATA {
    public int cbSize; public IntPtr hWnd; public uint uCallbackMessage; public uint uEdge; public RECT rc; public IntPtr lParam;
  }
  [DllImport("shell32.dll")] public static extern UIntPtr SHAppBarMessage(uint msg, ref APPBARDATA d);
  [DllImport("user32.dll")] public static extern IntPtr FindWindow(string c, string w);
  [DllImport("user32.dll")] public static extern IntPtr FindWindowEx(IntPtr p, IntPtr a, string c, string w);
  [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr h, int n);
  public static void Restore() {
    var d = new APPBARDATA(); d.cbSize = Marshal.SizeOf(d);
    d.hWnd = FindWindow("Shell_TrayWnd", null); d.lParam = (IntPtr)0;
    SHAppBarMessage(10, ref d);
    ShowWindow(FindWindow("Shell_TrayWnd", null), 5);
    IntPtr s = IntPtr.Zero;
    while ((s = FindWindowEx(IntPtr.Zero, s, "Shell_SecondaryTrayWnd", null)) != IntPtr.Zero) ShowWindow(s, 5);
  }
}
"@
    }
    [TBR]::Restore()
}

function Remove-GlazeWM {
    $had = $false
    $g = Get-Process -Name glazewm -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($g) { $had = $true; try { & $g.Path command wm-exit 2>$null | Out-Null } catch {}; Start-Sleep 2 }
    Get-Process -Name glazewm, glazewm-watcher, zebar -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    Get-CimInstance Win32_Process -Filter "Name='powershell.exe'" -ErrorAction SilentlyContinue |
        Where-Object { $_.CommandLine -like '*\.glzr\*' } |
        ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
    foreach ($p in @((Join-Path $startup 'GlazeWM.lnk'), (Join-Path $extras 'GlazeWM.lnk.backup'))) {
        if (Test-Path $p) { $had = $true; Remove-Item $p -Force }
    }
    $installed = (winget list --id glzr-io.glazewm -e --accept-source-agreements 2>$null | Out-String) -match 'glazewm'
    if ($installed) { $had = $true; winget uninstall --id glzr-io.glazewm -e --silent 2>$null | Out-Null }
    if ((winget list --id glzr-io.zebar -e --accept-source-agreements 2>$null | Out-String) -match 'zebar') { winget uninstall --id glzr-io.zebar -e --silent 2>$null | Out-Null }
    $glzr = Join-Path $HOME_ '.glzr'
    if (Test-Path $glzr) { $had = $true; Remove-Item $glzr -Recurse -Force -ErrorAction SilentlyContinue }
    if ($had) { Say 'Da go GlazeWM / Zebar cu.' 'Green' }
}

function Install-Apps {
    Refresh-Path
    if (-not (KC)) {
        Say 'Dang cai komorebi (1-2 phut)...'
        winget install --id LGUG2Z.komorebi -e --source winget --silent --accept-package-agreements --accept-source-agreements | Out-Null
    }
    if (-not (Get-Command whkd -ErrorAction SilentlyContinue)) {
        Say 'Dang cai whkd (phim tat)...'
        winget install --id LGUG2Z.whkd -e --source winget --silent --accept-package-agreements --accept-source-agreements | Out-Null
    }
    else {
        # phim bat/tat khi remote (.pause) can whkd ban moi -> cap nhat neu co ban moi hon
        winget upgrade --id LGUG2Z.whkd -e --source winget --silent --accept-package-agreements --accept-source-agreements 2>$null | Out-Null
    }
    Refresh-Path
    return [bool](KC)
}

# Lay file mau (komorebi.json, komorebi.bar.json, applications.json) bang quickstart vao thu muc tam
function Get-Templates {
    $need = -not (Test-Path $barTpl) -or -not (Test-Path (Join-Path $HOME_ 'applications.json'))
    if (-not $need) { return }
    Say 'Tai file cau hinh mau cua komorebi...'
    $tmp = Join-Path $env:TEMP ('komorebi-qs-' + (Get-Random)); New-Item -ItemType Directory -Force -Path $tmp | Out-Null
    $env:KOMOREBI_CONFIG_HOME = $tmp
    $p = Start-Process (KC) -ArgumentList 'quickstart' -WindowStyle Hidden -PassThru
    if (-not $p.WaitForExit(60000)) { try { $p.Kill() } catch {} }
    Remove-Item Env:\KOMOREBI_CONFIG_HOME -ErrorAction SilentlyContinue
    foreach ($fn in 'komorebi.bar.json', 'applications.json', 'komorebi.json') {
        $src = Get-ChildItem $tmp -Filter $fn -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1
        $dst = Join-Path $HOME_ ($(if ($fn -eq 'komorebi.json') { 'komorebi.template.json' } else { $fn }))
        if ($src -and -not (Test-Path $dst)) { Copy-Item $src.FullName $dst }
    }
    Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
}

function Write-BaseConfig {
    Backup $cfgFile
    if (Test-Path $cfgFile) { $cfg = Get-Content $cfgFile -Raw -Encoding UTF8 | ConvertFrom-Json }
    elseif (Test-Path (Join-Path $HOME_ 'komorebi.template.json')) { $cfg = Get-Content (Join-Path $HOME_ 'komorebi.template.json') -Raw -Encoding UTF8 | ConvertFrom-Json }
    else { $cfg = [pscustomobject]@{ '$schema' = 'https://raw.githubusercontent.com/LGUG2Z/komorebi/master/schema.json' } }
    if (Test-Path (Join-Path $HOME_ 'applications.json')) { Set-Prop $cfg 'app_specific_configuration_path' '$Env:USERPROFILE/applications.json' }
    elseif ($cfg.PSObject.Properties['app_specific_configuration_path']) { $cfg.PSObject.Properties.Remove('app_specific_configuration_path') }
    Set-Prop $cfg 'window_hiding_behaviour' 'Cloak'
    Set-Prop $cfg 'cross_monitor_move_behaviour' 'Insert'
    Set-Prop $cfg 'default_workspace_padding' ([int]1)
    Set-Prop $cfg 'default_container_padding' ([int]1)
    Set-Prop $cfg 'border' $true
    Set-Prop $cfg 'border_width' ([int]3)
    Set-Prop $cfg 'border_offset' ([int](-1))
    Set-Prop $cfg 'mouse_follows_focus' $true
    foreach ($p in @($cfg.PSObject.Properties)) { if ($p.Value -is [string] -and $p.Value -match '^-?\d+$') { $cfg.($p.Name) = [int]$p.Value } }
    foreach ($k in 'display_index_preferences', 'bar_configurations') { if ($cfg.PSObject.Properties[$k]) { $cfg.PSObject.Properties.Remove($k) } }
    # tam thoi: moi man 1 workspace (se chia lai sau khi komorebi nhan man)
    $tmpMon = @(); for ($i = 0; $i -lt 4; $i++) { $tmpMon += [pscustomobject]@{ workspaces = @([pscustomobject]@{ name = "tmp$i"; layout = 'BSP' }) } }
    Set-Prop $cfg 'monitors' $tmpMon
    Write-Json $cfg $cfgFile
}

# Mau thanh bar (theo schema komorebi-bar v0.1.41). $h = chieu cao; $null = mac dinh (50)
function New-Bar($index, $h) {
    $bh = if ($h) { [int]$h } else { 50 }
    $bar = [ordered]@{
        '$schema' = 'https://raw.githubusercontent.com/LGUG2Z/komorebi/v0.1.41/schema.bar.json'
        monitor = [ordered]@{ index = [int]$index; work_area_offset = [ordered]@{ left = 0; top = $bh; right = 0; bottom = $bh } }
        theme = [ordered]@{ palette = 'Base16'; name = 'Ashes'; accent = 'Base0D' }
        left_widgets = @(
            [ordered]@{ Komorebi = [ordered]@{
                workspaces = [ordered]@{ enable = $true; hide_empty_workspaces = $false }
                layout = [ordered]@{ enable = $true }
                focused_window = [ordered]@{ enable = $true; show_icon = $true }
            } }
        )
        right_widgets = @(
            [ordered]@{ Battery = [ordered]@{ enable = $true } },
            [ordered]@{ Date = [ordered]@{ enable = $true; format = 'DayDateMonthYear' } },
            [ordered]@{ Time = [ordered]@{ enable = $true; format = 'TwentyFourHour' } }
        )
    }
    if ($h) {
        $bar['height'] = [double]$h
        $bar['font_size'] = [double]11
        $bar['padding'] = [double]3
    }
    return [pscustomobject]$bar
}

# Chia workspace 1..10 cho cac man TRAI -> PHAI, ghi theo dung index cua komorebi; tao bar moi man
function Apply-Monitors($barHeight) {
    $kc = KC
    if (-not (Get-Process -Name komorebi -ErrorAction SilentlyContinue)) {
        & $kc start | Out-Null
        for ($i = 0; $i -lt 20 -and -not (Get-Process -Name komorebi -ErrorAction SilentlyContinue); $i++) { Start-Sleep 1 }
        Start-Sleep 2
    }
    try { $mons = @(((& $kc state | Out-String) | ConvertFrom-Json).monitors.elements) } catch { $mons = @() }
    if ($mons.Count -eq 0) { Say 'Komorebi khong chay -> khong doc duoc man hinh. Chup man hinh cua so nay gui Claude.' 'Red'; return $false }
    $N = $mons.Count

    Add-Type -AssemblyName System.Windows.Forms
    $screens = [System.Windows.Forms.Screen]::AllScreens
    $vis = @()
    for ($j = 0; $j -lt $N; $j++) {
        $m = $mons[$j]; $dn = "$($m.name)" -replace '^\\\\\.\\', ''; $scr = $null
        if ($dn) { $scr = $screens | Where-Object { $_.DeviceName -like "*\$dn" } | Select-Object -First 1 }
        if ($scr) { $x = $scr.Bounds.X; $y = $scr.Bounds.Y; $pt = $scr.Bounds.Height -gt $scr.Bounds.Width }
        else { $x = [int]$m.size.left; $y = [int]$m.size.top; $pt = $m.size.bottom -gt $m.size.right }
        $vis += [pscustomobject]@{ Idx = $j; X = $x; Y = $y; Portrait = $pt }
    }
    $rank = @($vis | Sort-Object X, Y)
    $base = [Math]::Floor($TOTAL / $N); $next = 1; $byIdx = @{}; $script:plan = @()
    for ($r = 0; $r -lt $N; $r++) {
        $o = $rank[$r]
        $count = if ($r -eq $N - 1) { $TOTAL - $next + 1 } else { $base }
        if ($count -lt 1) { $count = 1 }
        $layout = if ($o.Portrait) { 'Rows' } else { 'BSP' }
        $ws = @(); $nm = @()
        for ($k = 0; $k -lt $count; $k++) {
            $name = if ($next -le $TOTAL) { "$next" } else { "x$r$k" }
            $ws += [pscustomobject]@{ name = $name; layout = $layout }; $nm += $name; $next++
        }
        $byIdx[$o.Idx] = [pscustomobject]@{ workspaces = $ws }
        $script:plan += ("Man thu {0} tu trai sang{1}: workspace {2}" -f ($r + 1), $(if ($o.Portrait) { ' (DOC)' } else { '' }), ($nm -join ', '))
    }
    $monCfg = @(); for ($j = 0; $j -lt $N; $j++) { $monCfg += $byIdx[$j] }

    $cfg = Get-Content $cfgFile -Raw -Encoding UTF8 | ConvertFrom-Json
    Set-Prop $cfg 'monitors' $monCfg
    if ($cfg.PSObject.Properties['display_index_preferences']) { $cfg.PSObject.Properties.Remove('display_index_preferences') }

    # Thanh bar: moi man 1 thanh, tao tu mau co san trong file nay (khong phu thuoc quickstart)
    Get-ChildItem $HOME_ -Filter 'komorebi.bar.m*.json' -ErrorAction SilentlyContinue | Remove-Item -Force
    $paths = @()
    for ($j = 0; $j -lt $N; $j++) {
        Write-Json (New-Bar $j $barHeight) (Join-Path $HOME_ "komorebi.bar.m$j.json")
        $paths += ('$Env:USERPROFILE/komorebi.bar.m' + $j + '.json')
    }
    Set-Prop $cfg 'bar_configurations' $paths
    Write-Json $cfg $cfgFile

    $chk = & $kc check 2>&1 | Out-String
    if ($chk -match 'Error') { Write-Host $chk; Say 'Config bi loi (xem o tren).' 'Red'; return $false }
    return $true
}

function Write-Keys {
    Backup $whkdrc
    New-Item -ItemType Directory -Force -Path (Split-Path $whkdrc) | Out-Null
$restart = 'komorebic stop --whkd --bar & ping -n 3 127.0.0.1 >nul & start "" powershell -NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File "%USERPROFILE%\.config\komorebi-extras\komorebi-start.ps1"'
$k = @"
.shell cmd
# Alt+Shift+Esc: tam tat / bat lai TOAN BO phim tat o may nay (dung khi remote bang Parsec...).
# Khi tat, moi phim Alt+... di thang sang app dang mo, vd Parsec -> may ben kia.
.pause alt + shift + escape

# ==================================================================
#  Komorebi - phim tat theo bo MAC DINH cua i3 (Mod = Alt)
#  Tao tu dong boi Komorebi_Kieu_i3.bat
# ==================================================================

# ---- Mo ung dung ----
# i3: `$mod+Return = terminal
alt + return            : start wt
# i3: `$mod+d = dmenu  -> dat phim Alt+D cho PowerToys Command Palette (xem huong dan)
# Alt+Shift+D = hop Run cua Windows (giong dmenu_run)
alt + shift + d         : explorer shell:::{2559a1f8-21d7-11d4-bdaf-00c04f60b9f0}

# ---- Dong cua so ----
alt + q                 : komorebic close

# ---- Focus: h j k l (kieu vim) va mui ten ----
alt + h                 : komorebic focus left
alt + j                 : komorebic focus down
alt + k                 : komorebic focus up
alt + l                 : komorebic focus right
alt + left              : komorebic focus left
alt + down              : komorebic focus down
alt + up                : komorebic focus up
alt + right             : komorebic focus right

# ---- Di chuyen cua so ----
alt + shift + h         : komorebic move left
alt + shift + j         : komorebic move down
alt + shift + k         : komorebic move up
alt + shift + l         : komorebic move right
alt + shift + left      : komorebic move left
alt + shift + down      : komorebic move down
alt + shift + up        : komorebic move up
alt + shift + right     : komorebic move right

# ---- Chia / layout ----
# Alt+C xep theo cot, Alt+V xep theo hang (h da dung cho focus trai)
alt + c                 : komorebic change-layout columns
alt + v                 : komorebic change-layout rows
# i3: `$mod+s stacking, `$mod+w tabbed, `$mod+e toggle split
alt + s                 : komorebic stack-all
alt + w                 : komorebic stack-all
alt + e                 : komorebic unstack-all & komorebic change-layout bsp
# chuyen qua lai cac cua so trong 1 stack (tab)
alt + oem_4             : komorebic cycle-stack previous
alt + oem_6             : komorebic cycle-stack next

# ---- Toan man hinh / floating ----
alt + f                 : komorebic toggle-monocle
alt + shift + space     : komorebic toggle-float

# ---- "Scratchpad": an cua so ----
alt + shift + oem_minus : komorebic minimize

# ---- Resize (i3 dung che do Alt+R; whkd khong co che do nen dung Alt+Ctrl) ----
# kieu vim: h hep, j cao, k thap, l rong
alt + ctrl + h          : komorebic resize-axis horizontal decrease
alt + ctrl + j          : komorebic resize-axis vertical increase
alt + ctrl + k          : komorebic resize-axis vertical decrease
alt + ctrl + l          : komorebic resize-axis horizontal increase
alt + ctrl + left       : komorebic resize-axis horizontal decrease
alt + ctrl + down       : komorebic resize-axis vertical increase
alt + ctrl + up         : komorebic resize-axis vertical decrease
alt + ctrl + right      : komorebic resize-axis horizontal increase

# ---- Man hinh (output) ----
alt + oem_comma                 : komorebic cycle-monitor previous
alt + oem_period                : komorebic cycle-monitor next
alt + shift + oem_comma         : komorebic cycle-move-to-monitor previous
alt + shift + oem_period        : komorebic cycle-move-to-monitor next
# chuyen CA workspace sang man khac (i3: move workspace to output)
alt + ctrl + shift + h          : komorebic cycle-move-workspace-to-monitor previous
alt + ctrl + shift + l          : komorebic cycle-move-workspace-to-monitor next
alt + ctrl + shift + left       : komorebic cycle-move-workspace-to-monitor previous
alt + ctrl + shift + right      : komorebic cycle-move-workspace-to-monitor next

# ---- Workspace 1..10 (duy nhat, nhay thang du o man nao) ----

"@
foreach ($d in 1..$TOTAL) {
    $key = if ($d -eq 10) { '0' } else { "$d" }
    $k += ('alt + {0}                 : komorebic focus-named-workspace {1}' -f $key, $d) + "`r`n"
    $k += ('alt + shift + {0}         : komorebic move-to-named-workspace {1}' -f $key, $d) + "`r`n"
}
$k += @"

# ---- Quan ly ----
# i3: `$mod+Shift+c reload, `$mod+Shift+r restart, `$mod+Shift+e exit
alt + shift + c         : komorebic reload-configuration & taskkill /f /im whkd.exe & start /b whkd
alt + shift + r         : $restart
alt + shift + e         : komorebic stop --whkd --bar
# them (khong co trong i3): tam dung khi dung Blender/Reaper/Resolve/game
alt + shift + p         : komorebic toggle-pause
# xep lai cua so
alt + shift + w         : komorebic retile

# ---- Rieng cho may nay ----
alt + b                 : powershell -NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File "%USERPROFILE%\.config\komorebi-extras\toggle-taskbar.ps1"
"@
[IO.File]::WriteAllText($whkdrc, ($k -replace "(?<!\r)\n", "`r`n"), $utf8)
Say "Da ghi phim tat kieu i3: $whkdrc" 'Green'
}

function Write-Extras {
    New-Item -ItemType Directory -Force -Path $extras | Out-Null
    [IO.File]::WriteAllText($togglePs, $toggleText, $utf8)
    [IO.File]::WriteAllText($startPs, $startText, $utf8)
    $w = New-Object -ComObject WScript.Shell
    foreach ($p in @((Join-Path $startup 'Komorebi.lnk'), (Join-Path $programs 'Komorebi - Bat.lnk'))) {
        $s = $w.CreateShortcut($p)
        $s.TargetPath = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'
        $s.Arguments = "-NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File `"$startPs`""
        $s.WindowStyle = 7; $s.Save()
    }
    # neu komorebi tu tao loi tat rieng (khac ten), xoa de khong chay 2 lan
    Get-ChildItem $startup -Filter '*.lnk' -ErrorAction SilentlyContinue | Where-Object {
        $_.Name -ne 'Komorebi.lnk' -and ($w.CreateShortcut($_.FullName).TargetPath -like '*komorebic*')
    } | Remove-Item -Force -ErrorAction SilentlyContinue
    # Ban cu dung Ctrl+Alt+F11/F12 (loi tat Start Menu) -> da thay bang Alt+Shift+Esc trong whkd, xoa loi tat cu
    foreach ($n in 'Komorebi - Tat phim (remote).lnk', 'Komorebi - Bat lai phim.lnk') { Remove-Item (Join-Path $programs $n) -Force -ErrorAction SilentlyContinue }
}

function Write-PowerShortcuts {
    $w = New-Object -ComObject WScript.Shell
    $dir = Join-Path $programs 'Che do nguon'
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    Remove-Item (Join-Path $dir '*.lnk') -Force -ErrorAction SilentlyContinue
    $names = @{ '1' = '1 - Tiet kiem pin'; '2' = '2 - Van phong'; '3' = '3 - Choi game'; '4' = '4 - Do hoa - Video' }
    foreach ($k in $PLANS.Keys) {
        $s = $w.CreateShortcut((Join-Path $dir ('Nguon ' + $names[$k] + '.lnk')))
        $s.TargetPath = Join-Path $env:SystemRoot 'System32\powercfg.exe'
        $s.Arguments = '-setactive ' + $PLANS[$k]
        $s.Hotkey = 'CTRL+ALT+' + $k
        $s.WindowStyle = 7
        $s.IconLocation = (Join-Path $env:SystemRoot 'System32\powercpl.dll') + ',0'
        $s.Save()
    }
}

function Start-All {
    Stop-Komorebi
    Start-Process powershell -WindowStyle Hidden -ArgumentList "-NoProfile -ExecutionPolicy Bypass -File `"$startPs`""
    Start-Sleep 10
}

function Running { @(Get-Process -Name komorebi, whkd, komorebi-bar -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Name) }

# Thanh bar khong len -> thu 32px roi kich thuoc goc
function Ensure-Bars {
    $want = @(Get-ChildItem $HOME_ -Filter 'komorebi.bar.m*.json' -ErrorAction SilentlyContinue).Count
    if ($want -eq 0) { return }
    foreach ($h in @(32, $null)) {
        $have = @(Running | Where-Object { $_ -eq 'komorebi-bar' }).Count
        if ($have -ge $want) { return }
        Say ("Thanh bar chua hien ({0}/{1}) -> thu {2}" -f $have, $want, $(if ($h) { "cao $h px" } else { 'kich thuoc goc' })) 'Yellow'
        Stop-Komorebi
        if (Apply-Monitors $h) { Stop-Komorebi; Start-All }
    }
}

function Show-Result {
    Write-Host ''
    $script:plan | ForEach-Object { Write-Host "   $_" }
    $r = Running
    $bars = @($r | Where-Object { $_ -eq 'komorebi-bar' }).Count
    Write-Host ("   komorebi: {0} | phim tat (whkd): {1} | thanh bar: {2}" -f `
        $(if ($r -contains 'komorebi') { 'CO' } else { 'KHONG' }), $(if ($r -contains 'whkd') { 'CO' } else { 'KHONG' }), $bars)
    if ($r -contains 'komorebi' -and $r -contains 'whkd') { Say 'XONG!' 'Green' }
    else { Say 'Chua chay du. Chup man hinh cua so nay gui Claude.' 'Red' }
}

# ======================================================================
#  CHAY
# ======================================================================
function Do-UninstallKomorebi {
    Say 'Tat komorebi, whkd, thanh bar...'
    Stop-Komorebi
    Restore-Taskbar
    Say 'Xoa loi tat tu khoi dong + Start Menu'
    $w = New-Object -ComObject WScript.Shell
    foreach ($p in @((Join-Path $startup 'Komorebi.lnk'), (Join-Path $programs 'Komorebi - Bat.lnk'), (Join-Path $programs 'Komorebi - Tat phim (remote).lnk'), (Join-Path $programs 'Komorebi - Bat lai phim.lnk'))) { Remove-Item $p -Force -ErrorAction SilentlyContinue }
    Get-ChildItem $startup -Filter '*.lnk' -ErrorAction SilentlyContinue | Where-Object {
        ($w.CreateShortcut($_.FullName).TargetPath -like '*komorebi*') -or ($w.CreateShortcut($_.FullName).Arguments -like '*komorebi*')
    } | Remove-Item -Force -ErrorAction SilentlyContinue
    $kc = KC; if ($kc) { & $kc disable-autostart 2>&1 | Out-Null }
    Say 'Go cai dat komorebi + whkd'
    winget uninstall --id LGUG2Z.komorebi -e --silent 2>$null | Out-Null
    winget uninstall --id LGUG2Z.whkd -e --silent 2>$null | Out-Null
    Say 'Xoa file cau hinh'
    Get-ChildItem $HOME_ -File -ErrorAction SilentlyContinue | Where-Object {
        $_.Name -like 'komorebi*.json' -or $_.Name -eq 'applications.json'
    } | Remove-Item -Force -ErrorAction SilentlyContinue
    foreach ($p in @($whkdrc, $extras, (Join-Path $env:LOCALAPPDATA 'komorebi'), (Join-Path $env:APPDATA 'komorebi'))) {
        if (Test-Path $p) { Remove-Item $p -Recurse -Force -ErrorAction SilentlyContinue }
    }
    Refresh-Path
    if (KC) { Say 'Van con komorebi (co the can khoi dong lai may). Go tay trong Settings > Apps.' 'Yellow' }
    else { Say 'Da go sach komorebi + whkd. Taskbar Windows da tro lai binh thuong.' 'Green' }
}

$c = Read-Host '  Go Komorebi? Go "y" roi Enter de xac nhan'
if ($c -eq 'y') { Do-UninstallKomorebi } else { Write-Host '  Da huy.' }
