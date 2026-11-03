@echo off
setlocal
powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "$ErrorActionPreference='Stop'; & ([scriptblock]::Create((Invoke-WebRequest -UseBasicParsing 'https://raw.githubusercontent.com/wisjoycomm/setup-gateway/main/install.ps1').Content)) -RunSetup"
exit /b %errorlevel%
