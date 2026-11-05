$ErrorActionPreference = 'Stop'
$installer = Join-Path (Split-Path $PSScriptRoot) 'install.ps1'
$testRoot = Join-Path ([IO.Path]::GetTempPath()) ('wj-installer-test-' + [guid]::NewGuid().ToString('N'))
$processPathBefore = $env:Path
$userPathBefore = [Environment]::GetEnvironmentVariable('Path', 'User')
New-Item -ItemType Directory -Path $testRoot | Out-Null
try {
    foreach ($testCase in @('digest', 'checksums', 'reuse', 'mismatch', 'missing-checksum')) {
        & {
            param($testCase, $installer, $testRoot)
            $caseDir = Join-Path $testRoot $testCase
            New-Item -ItemType Directory -Path $caseDir | Out-Null
            $payload = [Text.Encoding]::UTF8.GetBytes('test binary, never executed')
            $hasher = [Security.Cryptography.SHA256]::Create()
            try { $expectedHash = ([BitConverter]::ToString($hasher.ComputeHash($payload))).Replace('-', '').ToLowerInvariant() }
            finally { $hasher.Dispose() }
            $arch = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
            $assetName = if ($arch -eq 'ARM64') { 'wj-windows-arm64.exe' } else { 'wj.exe' }
            $dest = Join-Path $caseDir 'wj.exe'
            [IO.File]::WriteAllText($dest, 'previous installation')
            if ($testCase -eq 'reuse') { [IO.File]::WriteAllBytes($dest, $payload) }
            function Invoke-RestMethod {
                param($Uri, $Headers)
                $digest = if ($testCase -in @('digest','reuse')) { "sha256:$expectedHash" } elseif ($testCase -eq 'mismatch') { 'sha256:' + ('0' * 64) } else { $null }
                $assets = @([pscustomobject]@{ name=$assetName; browser_download_url='https://example.invalid/binary'; digest=$digest })
                if ($testCase -eq 'checksums') { $assets += [pscustomobject]@{ name='checksums.txt'; browser_download_url='https://example.invalid/checksums' } }
                return [pscustomobject]@{ tag_name='v-test'; assets=$assets }
            }
            function Invoke-WebRequest {
                param($Uri, $OutFile, [switch]$UseBasicParsing)
                if ($Uri -eq 'https://example.invalid/binary') {
                    if ($testCase -eq 'reuse') { throw 'Downloaded an already installed release' }
                    [IO.File]::WriteAllBytes($OutFile, $payload); return
                }
                if ($Uri -eq 'https://example.invalid/checksums') { return [pscustomobject]@{ Content="$expectedHash  $assetName`n" } }
                throw 'Unexpected network request in test'
            }
            function Get-FileHash { throw 'Installer must not depend on Get-FileHash module auto-loading' }
            $failed = $false
            try { & $installer -Version 'v-test' -InstallDir $caseDir -NoPath }
            catch { $failed = $true }
            if ($testCase -in @('mismatch', 'missing-checksum')) {
                if (-not $failed) { throw "$testCase should have failed" }
                if ([IO.File]::ReadAllText($dest) -ne 'previous installation') { throw "$testCase replaced the old binary" }
            } else {
                if ($failed) { throw "$testCase unexpectedly failed" }
                if ([IO.File]::ReadAllText($dest) -ne 'test binary, never executed') { throw "$testCase installed wrong bytes" }
            }
            if (@(Get-ChildItem -LiteralPath $caseDir -Filter '*.tmp' -Force).Count) { throw "$testCase left a temporary download" }
            Write-Host "PASS installer $testCase"
        } $testCase $installer $testRoot
    }
    if ($env:Path -ne $processPathBefore -or [Environment]::GetEnvironmentVariable('Path', 'User') -ne $userPathBefore) { throw '-NoPath changed PATH' }
    Write-Host 'PASS installer leaves PATH unchanged with -NoPath'
} finally {
    $resolvedTestRoot = [IO.Path]::GetFullPath($testRoot)
    $tempPrefix = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\') + '\wj-installer-test-'
    if (-not $resolvedTestRoot.StartsWith($tempPrefix, [StringComparison]::OrdinalIgnoreCase)) { throw 'Unexpected cleanup directory' }
    Remove-Item -LiteralPath $resolvedTestRoot -Recurse -Force
}
