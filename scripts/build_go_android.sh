#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_BIN="${GO_BIN:-go}"
ANDROID_NDK_HOME="${ANDROID_NDK_HOME:-${ANDROID_HOME:-}/ndk/30.0.14904198}"
OUT_DIR="$ROOT_DIR/android/app/src/main/jniLibs"

if [[ ! -d "$ANDROID_NDK_HOME" ]]; then
  echo "ANDROID_NDK_HOME is required and must point to NDK 30" >&2
  exit 1
fi

mkdir -p "$OUT_DIR/arm64-v8a" "$OUT_DIR/armeabi-v7a" "$OUT_DIR/x86_64"

build_one() {
  local abi="$1"
  local goarch="$2"
  local goarm="${3:-}"
  local cc="$4"
  local target_dir="$OUT_DIR/$abi"
  (
    cd "$ROOT_DIR/core/go"
    CGO_ENABLED=1 GOOS=android GOARCH="$goarch" GOARM="$goarm" CC="$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/linux-x86_64/bin/$cc" \
      "$GO_BIN" build -trimpath -ldflags="-s -w" -o "$target_dir/libkvn-core.so" ./cmd/kvn-core
  )
}

build_one arm64-v8a arm64 "" aarch64-linux-android26-clang
build_one armeabi-v7a arm 7 armv7a-linux-androideabi26-clang
build_one x86_64 amd64 "" x86_64-linux-android26-clang
