<#
plan.ps1 - compact view of PLAN.md for the orchestrator, so it never has to read the whole file.

PLAN.md is one Markdown table and nothing else:
| ID | Task | Pri | SP | Status | Agent | Depends | Scope | Verify | Notes | Session |

Usage (from the main repo folder):
  pwsh -NoProfile -NonInteractive -File .claude/skills/orca-orchestrator/scripts/plan.ps1

Prints
  - the next free ID (from PLAN.md and PLAN-ARCHIVE.md), counts per status
  - ACTIVE: every task that is not TODO, with its line number and session
  - READY: TODO tasks that can start now, highest priority first, as launch commands
    (every Depends gone from PLAN.md, Scope not clashing with DOING/DONE tasks or with each other)
  - WAITING: TODO tasks that can't start yet, with the reason
  - FORMAT: lines that break the table format (fix them; `wj plan check` explains each)

Archive (PLAN-ARCHIVE.md, same columns + Commit + Archived), without reading the whole file:
  plan.ps1 -Archive                last 20 archived rows: id, commit, date, task
  plan.ps1 -Archive -Last 50       more rows
  plan.ps1 -Archive -Find T-12     the archived row(s) for one ID, or rows whose Task/Notes contain the text
#>
[CmdletBinding()]
param(
    [string]$Plan = 'PLAN.md',
    [switch]$Archive,
    [int]$Last = 20,
    [string]$Find = ''
)

$ErrorActionPreference = 'Stop'

$root = & git rev-parse --show-toplevel 2>$null
if ($LASTEXITCODE -eq 0 -and $root) { Set-Location -LiteralPath $root }

function Split-Row([string]$l) {
    $t = $l.Trim()
    if (-not ($t.StartsWith('|') -and $t.EndsWith('|')) -or $t.Length -lt 2) { return $null }
    return @($t.Substring(1, $t.Length - 2).Split('|') | ForEach-Object { $_.Trim() })
}

if ($Archive) {
    $file = 'PLAN-ARCHIVE.md'
    if (-not (Test-Path -LiteralPath $file)) { Write-Output "ERROR: $file not found"; exit 2 }
    # Only table rows with every column count; older free text above the table is skipped.
    $rows = [System.Collections.Generic.List[object]]::new()
    $n = 0
    foreach ($l in [System.IO.File]::ReadLines((Resolve-Path -LiteralPath $file).Path)) {
        $n++
        $c = Split-Row $l
        if (-not $c -or $c.Count -ne 13 -or $c[0] -notmatch '^[A-Za-z]+-\d+$') { continue }
        $rows.Add([pscustomobject]@{ Line = $n; Id = $c[0]; Title = $c[1]; Status = $c[4]; Notes = $c[9]; Commit = $c[11]; Date = $c[12] })
    }
    if ($Find) {
        $hits = @($rows | Where-Object { $_.Id -ieq $Find -or $_.Title -like "*$Find*" -or $_.Notes -like "*$Find*" })
    } else {
        $hits = @($rows | Select-Object -Last $Last)
    }
    Write-Output ("ARCHIVE: {0} rows{1}" -f $rows.Count, $(if ($Find) { ", matching '$Find': $($hits.Count)" } else { ", last $($hits.Count)" }))
    foreach ($r in $hits) {
        $title = if ($r.Title.Length -gt 70) { $r.Title.Substring(0, 67) + '...' } else { $r.Title }
        Write-Output ('  {0,-5} {1,-6} {2,-8} {3,-10} {4,-10} {5}' -f $r.Line, $r.Id, $r.Status, $r.Commit, $r.Date, $title)
    }
    exit 0
}

if (-not (Test-Path -LiteralPath $Plan)) { Write-Output "ERROR: $Plan not found"; exit 2 }

$cols = 'ID', 'Task', 'Pri', 'SP', 'Status', 'Agent', 'Depends', 'Scope', 'Verify', 'Notes', 'Session'
$lines = @(Get-Content -LiteralPath $Plan -Encoding utf8)
$tasks = [System.Collections.Generic.List[object]]::new()
$format = [System.Collections.Generic.List[string]]::new()

if ($lines.Count -lt 2 -or ((Split-Row $lines[0]) -join '|') -ne ($cols -join '|')) {
    $format.Add("line 1: header must be | $($cols -join ' | ') |  (old checkbox format? run: wj plan convert)")
}
for ($i = 2; $i -lt $lines.Count; $i++) {
    if (-not $lines[$i].Trim()) { continue }
    $c = Split-Row $lines[$i]
    if (-not $c) { $format.Add("line $($i + 1): text outside the table"); continue }
    if ($c.Count -ne $cols.Count) { $format.Add("line $($i + 1): $($c.Count) cells, need $($cols.Count)"); continue }
    $tasks.Add([pscustomobject]@{
        Line = $i + 1; Id = $c[0]; Title = $c[1]; Pri = $c[2]; SP = $c[3]; St = $c[4].ToUpper()
        Agent = if ($c[5]) { $c[5].ToLower() } else { 'claude' }
        Depends = @([regex]::Matches($c[6], '[A-Za-z]+-\d+') | ForEach-Object { $_.Value })
        Scope = @([regex]::Matches($c[7], '`([^`]+)`') | ForEach-Object { $_.Groups[1].Value.Trim() })
        Session = $c[10]
    })
}

# Next ID: highest T-number in PLAN.md and PLAN-ARCHIVE.md, plus one.
$max = 0
$ids = @($tasks | ForEach-Object { $_.Id })
if (Test-Path -LiteralPath 'PLAN-ARCHIVE.md') {
    $ids += @(Select-String -LiteralPath 'PLAN-ARCHIVE.md' -Pattern '\bT-\d+\b' -AllMatches | ForEach-Object { $_.Matches.Value })
}
foreach ($id in $ids) { if ($id -match '^T-(\d+)$' -and [int]$Matches[1] -gt $max) { $max = [int]$Matches[1] } }

# Scope overlap: compare the fixed part of each path (before any wildcard).
function Get-Prefix([string]$s) {
    $s = ($s -replace '\\', '/') -replace '^\./', ''
    $cut = $s.IndexOfAny([char[]]'*?[')
    if ($cut -ge 0) { $s = $s.Substring(0, $cut) }
    return $s.TrimEnd('/')
}
function Find-Clash([string[]]$a, [string[]]$b) {
    if ($a.Count -eq 0 -or $b.Count -eq 0) { return '(no Scope)' }
    foreach ($x in $a) {
        $px = Get-Prefix $x
        foreach ($y in $b) {
            $py = Get-Prefix $y
            if ($px -eq '' -or $py -eq '') { return "$x ~ $y" }
            if ($px -ieq $py -or $px.StartsWith("$py/", 'OrdinalIgnoreCase') -or $py.StartsWith("$px/", 'OrdinalIgnoreCase')) {
                return "$x ~ $y"
            }
        }
    }
    return $null
}
function Short([string]$s, [int]$n) { if ($s.Length -gt $n) { $s.Substring(0, $n - 3) + '...' } else { $s } }

$byId = @{}
foreach ($t in $tasks) { $byId[$t.Id] = $t }

Write-Output ('NEXT ID: T-{0:D2}' -f ($max + 1))
$count = @{}
foreach ($t in $tasks) { $count[$t.St] = 1 + [int]$count[$t.St] }
Write-Output ("COUNTS: todo {0}, doing {1}, done {2}, blocked {3}, partial {4}" -f [int]$count['TODO'], [int]$count['DOING'], [int]$count['DONE'], [int]$count['BLOCKED'], [int]$count['PARTIAL'])

$active = @($tasks | Where-Object { $_.St -ne 'TODO' })
if ($active.Count) {
    Write-Output 'ACTIVE (line, id, status, session, task):'
    foreach ($t in $active) {
        Write-Output ('  {0,-4} {1,-6} {2,-8} {3,-14} {4}' -f $t.Line, $t.Id, $t.St, $t.Session, (Short $t.Title 60))
    }
}

# Files of DOING tasks, and of DONE tasks not merged + archived yet, are taken.
$busy = @($tasks | Where-Object { $_.St -eq 'DOING' -or $_.St -eq 'DONE' })
$ready = [System.Collections.Generic.List[object]]::new()
$waiting = [System.Collections.Generic.List[string]]::new()

$priRank = @{ P0 = 0; P1 = 1; P2 = 2; P3 = 3 }
$todo = @($tasks | Where-Object { $_.St -eq 'TODO' } | Sort-Object @{ Expression = { if ($priRank.ContainsKey($_.Pri)) { $priRank[$_.Pri] } else { 9 } } }, Line)
foreach ($t in $todo) {
    $open = @($t.Depends | Where-Object { $byId.ContainsKey($_) } | ForEach-Object { "$_($($byId[$_].St))" })
    if ($open.Count) { $waiting.Add("$($t.Id) needs $($open -join ',') merged + archived"); continue }
    $reason = $null
    foreach ($o in @($busy) + @($ready)) {
        $c = Find-Clash $t.Scope $o.Scope
        if ($c) { $reason = "$($t.Id) scope clash with $($o.Id): $c"; break }
    }
    if ($reason) { $waiting.Add($reason); continue }
    $ready.Add($t)
}

Write-Output 'READY (highest priority first):'
if ($ready.Count -eq 0) { Write-Output '  (none)' }
foreach ($t in $ready) {
    $cmd = if ($t.Agent -eq 'codex') { "`$orca-stateless-worker $($t.Id)" } else { "/orca-stateless-worker $($t.Id)" }
    Write-Output ('  {0,-30} # line {1}, {2} {3} SP{4}: {5}' -f $cmd, $t.Line, $t.Agent, $t.Pri, $t.SP, (Short $t.Title 50))
}
if ($waiting.Count) {
    Write-Output 'WAITING:'
    $waiting | ForEach-Object { Write-Output "  $_" }
}
foreach ($f in $format) { Write-Output "FORMAT: $f" }
