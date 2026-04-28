#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="$ROOT_DIR/.native/bin"
mkdir -p "$BIN_DIR"

echo "Fetch/build xray-core latest, Arti >= 0.41, and hev-socks5-tunnel here."
echo "Place platform binaries in private app assets or jniLibs before release packaging."
echo "Expected runtime names: xray-core, arti, hev-socks5-tunnel."
