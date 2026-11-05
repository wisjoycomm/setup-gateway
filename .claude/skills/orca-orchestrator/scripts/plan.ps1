<#
plan.ps1 - compact view of PLAN.md for the orchestrator, so it never has to read the whole file.

Usage (from the main repo folder):
  pwsh -NoProfile -NonInteractive -File .claude/skills/orca-orchestrator/scripts/plan.ps1

Prints
  - the Next ID and one line per running / done-unmerged / blocked task, with its line number
  - counts per status
  - READY: tasks that can start now, as launch commands for their agent
    (status [ ], every Depends done or archived, Scope not clashing with running/unmerged tasks or with each other)
  - WAITING: todo tasks that can't start yet, with the reason
#>
[CmdletBinding()]
param([string]$Plan = 'PLAN.md')

$ErrorActionPreference = 'Stop'

$root = & git rev-parse --show-toplevel 2>$null
if ($LASTEXITCODE -eq 0 -and $root) { Set-Location -LiteralPath $root }
if (-not (Test-Path -LiteralPath $Plan)) { Write-Output "ERROR: $Plan not found"; exit 2 }

$lines = @(Get-Content -LiteralPath $Plan -Encoding utf8)
$nextId = ''
$tasks = [System.Collections.Generic.List[object]]::new()
$cur = $null
$inComment = $false

for ($i = 0; $i -lt $lines.Count; $i++) {
    $l = $lines[$i]
    if ($l -match '<!--') { $inComment = $true }
    if ($inComment) { if ($l -match '-->') { $inComment = $false }; continue }
    if (-not $nextId -and $l -match '^\s*Next ID:\s*(\S+)') { $nextId = $Matches[1]; continue }
    if ($l -match '^\s*-\s*\[(.)\]\s*\*\*([A-Za-z]+-\d+)\*\*\s*:?\s*(.*)$') {
        $cur = [pscustomobject]@{
            Id = $Matches[2]; St = $Matches[1]; Title = $Matches[3].Trim(); Line = $i + 1
            Agent = 'claude'; Scope = @(); Depends = @()
        }
        $tasks.Add($cur)
        continue
    }
    if ($l -match '^#') { $cur = $null; continue }
    if (-not $cur) { continue }
    if ($l -match '^\s*-\s*\*\*Agent\*\*\s*:\s*(\S*)') {
        if ($Matches[1]) { $cur.Agent = $Matches[1].ToLower() }
    } elseif ($l -match '^\s*-\s*\*\*Scope\*\*\s*:(.*)$') {
        $cur.Scope = @([regex]::Matches($Matches[1], '`([^`]+)`') | ForEach-Object { $_.Groups[1].Value.Trim() })
    } elseif ($l -match '^\s*-\s*\*\*Depends\*\*\s*:(.*)$') {
        $cur.Depends = @([regex]::Matches($Matches[1], '[A-Za-z]+-\d+') | ForEach-Object { $_.Value })
    }
}

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
            if ($px -eq $py -or $px.StartsWith("$py/", 'OrdinalIgnoreCase') -or $py.StartsWith("$px/", 'OrdinalIgnoreCase') -or $px -ieq $py) {
                return "$x ~ $y"
            }
        }
    }
    return $null
}

$byId = @{}
foreach ($t in $tasks) { $byId[$t.Id] = $t }

function Short([string]$s, [int]$n) { if ($s.Length -gt $n) { $s.Substring(0, $n - 3) + '...' } else { $s } }

Write-Output "NEXT ID: $nextId"
$active = @($tasks | Where-Object { $_.St -ne ' ' })
if ($active.Count) {
    Write-Output 'ACTIVE (line, id, status, agent, title):'
    foreach ($t in $active) {
        Write-Output ('  {0,-4} {1,-6} [{2}] {3,-7} {4}' -f $t.Line, $t.Id, $t.St, $t.Agent, (Short $t.Title 60))
    }
}
$count = @{ ' ' = 0; '~' = 0; 'x' = 0; '!' = 0 }
foreach ($t in $tasks) { if ($count.ContainsKey($t.St)) { $count[$t.St]++ } }
Write-Output ("COUNTS: todo {0}, running {1}, done-unmerged {2}, blocked {3}" -f $count[' '], $count['~'], $count['x'], $count['!'])

# Tasks whose files are taken: running, or done but not merged/archived yet.
$busy = @($tasks | Where-Object { $_.St -eq '~' -or $_.St -eq 'x' })
$ready = [System.Collections.Generic.List[object]]::new()
$waiting = [System.Collections.Generic.List[string]]::new()

foreach ($t in ($tasks | Where-Object { $_.St -eq ' ' })) {
    $open = @($t.Depends | Where-Object { $byId.ContainsKey($_) -and $byId[$_].St -ne 'x' })
    $unmerged = @($t.Depends | Where-Object { $byId.ContainsKey($_) -and $byId[$_].St -eq 'x' })
    if ($open.Count) { $waiting.Add("$($t.Id) needs $($open -join ',')"); continue }
    if ($unmerged.Count) { $waiting.Add("$($t.Id) needs $($unmerged -join ',') merged + archived"); continue }
    $reason = $null
    foreach ($o in @($busy) + @($ready)) {
        $c = Find-Clash $t.Scope $o.Scope
        if ($c) { $reason = "$($t.Id) scope clash with $($o.Id): $c"; break }
    }
    if ($reason) { $waiting.Add($reason); continue }
    $ready.Add($t)
}

Write-Output 'READY:'
if ($ready.Count -eq 0) { Write-Output '  (none)' }
foreach ($t in $ready) {
    $cmd = if ($t.Agent -eq 'codex') { "`$orca-stateless-worker $($t.Id)" } else { "/orca-stateless-worker $($t.Id)" }
    Write-Output ('  {0,-30} # line {1}, {2}: {3}' -f $cmd, $t.Line, $t.Agent, (Short $t.Title 50))
}
if ($waiting.Count) {
    Write-Output 'WAITING:'
    $waiting | ForEach-Object { Write-Output "  $_" }
}
