param([switch]$All)
$ErrorActionPreference = 'Stop'
$buildRoot = $PSScriptRoot
$dist = Join-Path $buildRoot 'dist'
New-Item -ItemType Directory -Path $dist -Force | Out-Null
$savedOS = $env:GOOS
$savedArch = $env:GOARCH
$savedCGO = $env:CGO_ENABLED
try {
    $targets = @(@{ OS='windows'; Arch='amd64'; Name='wj.exe' })
    if ($All) {
        $targets += @(
            @{ OS='windows'; Arch='arm64'; Name='wj-windows-arm64.exe' },
            @{ OS='darwin'; Arch='amd64'; Name='wj-macos-amd64' },
            @{ OS='darwin'; Arch='arm64'; Name='wj-macos-arm64' },
            @{ OS='linux'; Arch='amd64'; Name='wj-linux-amd64' },
            @{ OS='linux'; Arch='arm64'; Name='wj-linux-arm64' }
        )
    }
    foreach ($target in $targets) {
        $env:GOOS = $target.OS
        $env:GOARCH = $target.Arch
        $env:CGO_ENABLED = '0'
        go -C $buildRoot build -trimpath '-ldflags=-s -w' -o (Join-Path $dist $target.Name) .
        if ($LASTEXITCODE -ne 0) { throw "Build failed: $($target.Name)" }
    }
    Copy-Item -LiteralPath (Join-Path $buildRoot 'README.md'),(Join-Path $buildRoot 'HUONG-DAN.md'),(Join-Path $buildRoot 'Run-Setup.cmd') -Destination $dist -Force
    $zipPath = Join-Path $dist 'wj-windows-amd64.zip'
    Compress-Archive -LiteralPath (Join-Path $dist 'wj.exe'),(Join-Path $dist 'README.md'),(Join-Path $dist 'HUONG-DAN.md'),(Join-Path $dist 'Run-Setup.cmd') -DestinationPath $zipPath -Force
    $hashes = Get-ChildItem -LiteralPath $dist -File | Where-Object Name -Like 'wj*' | Sort-Object Name | ForEach-Object {
        '{0}  {1}' -f (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant(), $_.Name
    }
    [IO.File]::WriteAllLines((Join-Path $dist 'checksums.txt'), [string[]]$hashes, [Text.UTF8Encoding]::new($false))
    Write-Host "Built: $zipPath"
} finally {
    $env:GOOS = $savedOS
    $env:GOARCH = $savedArch
    $env:CGO_ENABLED = $savedCGO
}
