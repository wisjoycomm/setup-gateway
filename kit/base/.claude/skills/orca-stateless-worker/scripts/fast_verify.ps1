<#
fast_verify.ps1 - verify ONE PLAN.md task inside an Orca worktree.

Usage (from the repo/worktree root):
  pwsh -NoProfile -NonInteractive -File .claude/skills/orca-stateless-worker/scripts/fast_verify.ps1 -TaskId T-01

What it does
  1. Lists changed files (modified, staged and new) and warns about files outside the task's Scope.
  2. Runs the task's Verify command from PLAN.md. If Verify is empty it auto-detects:
     Unity EditMode tests > package.json test script (or tsc --noEmit) > pytest > go test.
  3. Prints only the tail of the output, then one final status line.

Final line / exit code
  VERIFY: PASS                    exit 0
  VERIFY: FAIL (...)              exit 1   counts as a strike
  VERIFY: NOT_VERIFIED (reason)   exit 2   nothing was checked: not a pass, not a strike

Optional environment variables
  UNITY_EXE            full path to the Unity editor executable (skips auto-detection)
  UNITY_TEST_PLATFORM  EditMode (default) or PlayMode
  VERIFY_TAIL          lines of output shown on failure (default 60)
#>
[CmdletBinding()]
param(
    [string]$TaskId = '',
    [string]$Plan = 'PLAN.md'
)

$ErrorActionPreference = 'Stop'
$Tail = 60
if ($env:VERIFY_TAIL -match '^\d+$') { $Tail = [int]$env:VERIFY_TAIL }

# Child processes: run once (no watch mode), no colors, no pagers, UTF-8 output.
$env:CI = 'true'
$env:NO_COLOR = '1'
$env:FORCE_COLOR = '0'
$env:GIT_PAGER = 'cat'
$env:PAGER = 'cat'
$env:PYTHONUTF8 = '1'
$env:PYTHONIOENCODING = 'utf-8'
$env:PYTHONDONTWRITEBYTECODE = '1'   # no __pycache__ noise in git status
try { [Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false) } catch { }
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)

# ---------------------------------------------------------------- helpers

function Finish([string]$Status, [int]$Code) {
    Write-Output "VERIFY: $Status"
    exit $Code
}

function Skip([string]$Reason) { Finish "NOT_VERIFIED ($Reason)" 2 }

function Write-Tail([object[]]$Lines, [int]$Count) {
    $clean = @(
        $Lines | ForEach-Object {
            $line = ([string]$_ -split "`r")[-1]                        # keep the last state of \r progress lines
            ($line -replace "`e\[[0-9;?]*[ -/]*[@-~]", '').TrimEnd()    # strip ANSI escapes
        } | Where-Object { $_ -ne '' }
    )
    if ($clean.Count -gt $Count) {
        Write-Output "... ($($clean.Count - $Count) earlier lines hidden)"
        $clean = $clean[-$Count..-1]
    }
    $clean | ForEach-Object { Write-Output $_ }
}

function Test-InScope([string]$File, [string[]]$Scope) {
    $f = $File -replace '\\', '/'
    $candidates = @($f)
    if ($f.EndsWith('.meta')) { $candidates += $f.Substring(0, $f.Length - 5) }   # Unity: X.meta follows X
    foreach ($raw in $Scope) {
        $s = ($raw -replace '\\', '/') -replace '^\./', ''
        foreach ($c in $candidates) {
            if ($s -match '[\*\?\[]') {
                if ($c -like $s) { return $true }
            } elseif ($s.EndsWith('/')) {
                if ($c.StartsWith($s, [StringComparison]::OrdinalIgnoreCase)) { return $true }
            } elseif ($c -eq $s -or $c.StartsWith("$s/", [StringComparison]::OrdinalIgnoreCase)) {
                return $true
            }
        }
    }
    return $false
}

function Invoke-Check([string]$Command) {
    Write-Output "--- check: $Command ---"
    if ($IsWindows) {
        $out = & cmd.exe /d /s /c "($Command) 2>&1"
    } else {
        $out = & bash -c "{ $Command ; } 2>&1"
    }
    $code = $LASTEXITCODE
    if ($code -eq 0) {
        Write-Tail $out 3
        Finish 'PASS' 0
    }
    Write-Tail $out $Tail
    Finish "FAIL (exit $code)" 1
}

function Find-UnityEditor {
    if ($env:UNITY_EXE) { return $env:UNITY_EXE }
    $versionFile = 'ProjectSettings/ProjectVersion.txt'
    $m = Select-String -LiteralPath $versionFile -Pattern '^m_EditorVersion:\s*(\S+)' | Select-Object -First 1
    if (-not $m) { return $null }
    $ver = $m.Matches[0].Groups[1].Value

    # Default Unity Hub install folder, plus a custom one if set in Unity Hub.
    if ($IsWindows) {
        $bases = @("$env:ProgramFiles\Unity\Hub\Editor")
        $hubCfg = Join-Path $env:APPDATA 'UnityHub\secondaryInstallPath.json'
        $rel = 'Editor\Unity.exe'
    } elseif ($IsMacOS) {
        $bases = @('/Applications/Unity/Hub/Editor')
        $hubCfg = Join-Path $HOME 'Library/Application Support/UnityHub/secondaryInstallPath.json'
        $rel = 'Unity.app/Contents/MacOS/Unity'
    } else {
        $bases = @((Join-Path $HOME 'Unity/Hub/Editor'))
        $hubCfg = Join-Path $HOME '.config/UnityHub/secondaryInstallPath.json'
        $rel = 'Editor/Unity'
    }
    if (Test-Path -LiteralPath $hubCfg) {
        try {
            $custom = Get-Content -LiteralPath $hubCfg -Raw | ConvertFrom-Json
            if ($custom) { $bases = @([string]$custom) + $bases }
        } catch { }
    }
    foreach ($b in $bases) {
        $exe = Join-Path (Join-Path $b $ver) $rel
        if (Test-Path -LiteralPath $exe) { return $exe }
    }
    return "missing:$ver"
}

function Invoke-UnityCheck([string]$Root) {
    $unity = Find-UnityEditor
    if (-not $unity) { Skip 'cannot read m_EditorVersion from ProjectSettings/ProjectVersion.txt' }
    if ($unity.StartsWith('missing:')) { Skip "Unity $($unity.Substring(8)) not found - set UNITY_EXE to the editor path" }
    if (-not (Test-Path -LiteralPath $unity)) { Skip "UNITY_EXE not found: $unity" }

    $platform = if ($env:UNITY_TEST_PLATFORM) { $env:UNITY_TEST_PLATFORM } else { 'EditMode' }
    $tmp = Join-Path ([IO.Path]::GetTempPath()) ('orca-verify-' + [guid]::NewGuid().ToString('N').Substring(0, 8))
    New-Item -ItemType Directory -Path $tmp | Out-Null
    $results = Join-Path $tmp 'results.xml'
    $log = Join-Path $tmp 'unity.log'

    Write-Output "--- check: Unity $platform tests (batchmode) ---"
    $psi = [System.Diagnostics.ProcessStartInfo]::new($unity)
    foreach ($a in @('-batchmode', '-nographics', '-projectPath', $Root, '-runTests',
                     '-testPlatform', $platform, '-testResults', $results, '-logFile', $log)) {
        $psi.ArgumentList.Add($a)
    }
    $psi.UseShellExecute = $false
    $proc = [System.Diagnostics.Process]::Start($psi)
    $proc.WaitForExit()            # waits for Unity itself, not its helper processes
    $code = $proc.ExitCode

    $logText = if (Test-Path -LiteralPath $log) { Get-Content -LiteralPath $log -Raw } else { '' }

    if (Test-Path -LiteralPath $results) {
        [xml]$xml = Get-Content -LiteralPath $results -Raw
        $run = $xml.'test-run'
        $failedCases = @($xml.SelectNodes("//test-case[@result='Failed']"))
        Write-Output "tests: $($run.total) total, $($run.passed) passed, $($run.failed) failed"
        if ([int]$run.total -eq 0) { Write-Output 'NOTE no tests found - only compilation was checked' }
        if ($failedCases.Count -eq 0 -and $code -eq 0) {
            Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
            Finish 'PASS' 0
        }
        foreach ($tc in ($failedCases | Select-Object -First 10)) {
            $msg = ''
            $node = $tc.SelectSingleNode('failure/message')
            if ($node) { $msg = ($node.InnerText.Trim() -split "`n")[0].Trim() }
            Write-Output "FAILED $($tc.GetAttribute('fullname')): $msg"
        }
        if ($failedCases.Count -gt 10) { Write-Output "... and $($failedCases.Count - 10) more" }
        Write-Output "full results: $results"
        Finish "FAIL ($($failedCases.Count) Unity tests failed, exit $code)" 1
    }

    # No results file: Unity stopped before running tests.
    if ($logText -match 'Multiple Unity instances cannot open the same project|another Unity instance is running') {
        Skip 'this project folder is already open in a Unity Editor - close it and rerun'
    }
    if ($logText -match 'No valid Unity Editor license|License is not active|Failed to activate') {
        Skip 'Unity license problem - open Unity Hub and sign in'
    }
    $compileErrors = @([regex]::Matches($logText, '(?m)^.*error CS\d{4}:.*$') |
                       ForEach-Object { $_.Value.Trim() } | Select-Object -Unique)
    if ($compileErrors.Count -gt 0) {
        $compileErrors | Select-Object -First 15 | ForEach-Object { Write-Output $_ }
        if ($compileErrors.Count -gt 15) { Write-Output "... and $($compileErrors.Count - 15) more" }
        Write-Output "full log: $log"
        Finish "FAIL ($($compileErrors.Count) C# compile errors)" 1
    }
    if ($code -eq 0) { Skip "Unity exited 0 but wrote no test results - see $log" }
    Write-Tail ($logText -split "`n") 40
    Write-Output "full log: $log"
    Finish "FAIL (Unity exit $code, no test results)" 1
}

# ---------------------------------------------------------------- main

try {
    $root = & git rev-parse --show-toplevel 2>$null
    if ($LASTEXITCODE -ne 0 -or -not $root) { Skip 'not inside a git repository' }
    Set-Location -LiteralPath $root

    # 1. Read the task block from PLAN.md
    $scope = @()
    $verify = ''
    if ($TaskId) {
        if ($TaskId -notmatch '^[A-Za-z]+-\d+$') { Skip "bad TaskId '$TaskId' (expected e.g. T-01)" }
        if (-not (Test-Path -LiteralPath $Plan)) { Skip "$Plan not found - commit it before launching workers" }
        # PLAN.md is one table: | ID | Task | Pri | SP | Status | Agent | Depends | Scope | Verify | Notes | Session |
        $row = $null
        foreach ($l in @(Get-Content -LiteralPath $Plan -Encoding utf8)) {
            $t = $l.Trim()
            if (-not ($t.StartsWith('|') -and $t.EndsWith('|'))) { continue }
            $c = @($t.Substring(1, $t.Length - 2).Split('|') | ForEach-Object { $_.Trim() })
            if ($c.Count -eq 11 -and $c[0] -eq $TaskId) { $row = $c; break }
        }
        if (-not $row) { Skip "task $TaskId not found in $Plan (one table row per task; old format? run: wj plan convert)" }
        $scope = @([regex]::Matches($row[7], '`([^`]+)`') | ForEach-Object { $_.Groups[1].Value.Trim() })
        $vm = [regex]::Match($row[8], '`([^`]+)`')
        if ($vm.Success) { $verify = $vm.Groups[1].Value.Trim() }
    }

    # 2. Changed files (tracked + staged + untracked) and scope check
    $label = if ($TaskId) { " ($TaskId)" } else { '' }
    Write-Output "--- changes$label ---"
    $status = @(& git -c core.quotepath=false status --porcelain --untracked-files=all)
    if ($LASTEXITCODE -ne 0) { Skip 'git status failed' }
    if ($status.Count -eq 0) { Skip 'no changes in this worktree' }
    $status | Select-Object -First 30 | ForEach-Object { Write-Output $_ }
    if ($status.Count -gt 30) { Write-Output "... and $($status.Count - 30) more (check .gitignore?)" }

    $changed = @(
        foreach ($s in $status) {
            if ($s.Length -lt 4) { continue }
            $p = $s.Substring(3)
            if ($p -match ' -> ') { $p = ($p -split ' -> ')[-1] }
            $p = $p.Trim('"')
            if ($p -notmatch '^\.tmp/') { $p }   # scratch folder from AGENTS.md
        }
    )
    if ($changed -contains $Plan) {
        Write-Output "WARN $Plan was edited - revert it (only the orchestrator edits $Plan)"
    }
    if ($TaskId -and $scope.Count -gt 0) {
        foreach ($c in $changed) {
            if ($c -eq $Plan) { continue }
            if (-not (Test-InScope $c $scope)) { Write-Output "WARN out-of-scope: $c" }
        }
    } elseif ($TaskId) {
        Write-Output "NOTE $TaskId has no Scope - scope check skipped"
    }

    # 3. Run the check
    $nodeCmd = $verify -match '^(npm|npx|pnpm|yarn|bun)(\s|$)'
    $isUnity = Test-Path -LiteralPath 'ProjectSettings/ProjectVersion.txt'
    $isNode = Test-Path -LiteralPath 'package.json'
    $isPython = (Test-Path 'pyproject.toml') -or (Test-Path 'pytest.ini') -or (Test-Path 'setup.cfg') -or (Test-Path 'setup.py')

    $pm = 'npm'
    if (Test-Path 'pnpm-lock.yaml') { $pm = 'pnpm' }
    elseif (Test-Path 'yarn.lock') { $pm = 'yarn' }
    elseif ((Test-Path 'bun.lock') -or (Test-Path 'bun.lockb')) { $pm = 'bun' }
    $install = switch ($pm) {
        'pnpm' { 'pnpm install --frozen-lockfile' }
        'yarn' { 'yarn install --frozen-lockfile' }
        'bun'  { 'bun install --frozen-lockfile' }
        default { if (Test-Path 'package-lock.json') { 'npm ci' } else { 'npm install' } }
    }
    $needsModules = $isNode -and ($nodeCmd -or (-not $verify -and -not $isUnity))
    if ($needsModules -and -not (Test-Path -LiteralPath 'node_modules')) {
        Skip "node_modules missing in this worktree - run '$install' once, then rerun"
    }

    if ($verify) { Invoke-Check $verify }

    if ($isUnity) { Invoke-UnityCheck $root }

    if ($isNode) {
        $pkg = Get-Content -LiteralPath 'package.json' -Raw | ConvertFrom-Json
        $testScript = $null
        if ($pkg.PSObject.Properties['scripts'] -and $pkg.scripts.PSObject.Properties['test']) { $testScript = $pkg.scripts.test }
        if ($testScript -and $testScript -notmatch 'no test specified') {
            $cmd = switch ($pm) { 'npm' { 'npm test --silent' } 'bun' { 'bun run test' } default { "$pm test" } }
            Invoke-Check $cmd
        }
        if (Test-Path 'tsconfig.json') {
            $cmd = switch ($pm) { 'npm' { 'npx --no-install tsc --noEmit -p .' } 'bun' { 'bunx tsc --noEmit -p .' } default { "$pm exec tsc --noEmit -p ." } }
            Invoke-Check $cmd
        }
        Skip 'package.json has no test script and no tsconfig.json - set Verify in PLAN.md'
    }

    if ($isPython) {
        $venvPy = @('.venv/Scripts/python.exe', '.venv/bin/python') | Where-Object { Test-Path $_ } | Select-Object -First 1
        if ($venvPy) {
            $venvPy = $venvPy -replace '/', [IO.Path]::DirectorySeparatorChar
            Invoke-Check "`"$venvPy`" -m pytest -q --tb=short --maxfail=1"
        }
        if (Get-Command pytest -ErrorAction SilentlyContinue) { Invoke-Check 'pytest -q --tb=short --maxfail=1' }
        Invoke-Check 'python -m pytest -q --tb=short --maxfail=1'
    }

    if (Test-Path -LiteralPath 'go.mod') { Invoke-Check 'go test ./...' }

    Skip "no Verify command in $Plan and no known stack (Unity / package.json / pytest / go)"
}
catch {
    Write-Output "ERROR: $($_.Exception.Message)"
    Skip 'verify script error'
}
