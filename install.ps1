$ErrorActionPreference = "Stop"
$repo = "wisjoycomm/setup-gateway"
$dir = Join-Path $env:USERPROFILE ".local\bin"
New-Item -ItemType Directory -Force -Path $dir | Out-Null
$dest = Join-Path $dir "setup-gateway.exe"
Invoke-WebRequest "https://github.com/$repo/releases/latest/download/setup-gateway.exe" -OutFile $dest
$path = [Environment]::GetEnvironmentVariable("Path", "User")
if ($path -notlike "*$dir*") {
    [Environment]::SetEnvironmentVariable("Path", "$path;$dir", "User")
}
Write-Host "installed: $dest"
Write-Host "open a new terminal and run: setup-gateway"
