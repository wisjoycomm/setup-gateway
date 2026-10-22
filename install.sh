#!/bin/sh
set -e
repo="wisjoycomm/setup-gateway"
os=$(uname -s)
arch=$(uname -m)
case "$os-$arch" in
  Darwin-arm64) file="setup-gateway-macos-arm64" ;;
  Darwin-x86_64) file="setup-gateway-macos-amd64" ;;
  Linux-x86_64) file="setup-gateway-linux-amd64" ;;
  Linux-aarch64) file="setup-gateway-linux-arm64" ;;
  *) echo "no build for $os $arch"; exit 1 ;;
esac
dir="$HOME/.local/bin"
mkdir -p "$dir"
curl -fsSL "https://github.com/$repo/releases/latest/download/$file" -o "$dir/setup-gateway"
chmod +x "$dir/setup-gateway"
case ":$PATH:" in
  *":$dir:"*) ;;
  *) echo "add to your shell profile: export PATH=\"$dir:\$PATH\"" ;;
esac
echo "installed: $dir/setup-gateway"
echo "open a new terminal and run: setup-gateway"
