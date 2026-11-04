#!/bin/sh
# ==============================================================================
# build_synology.sh — MusicConvert Production Binary Build Pipeline (Rust)
# Builds native release and Linux cross-compiled binaries for Synology NAS.
# ==============================================================================
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT_DIR"

export PATH="/opt/homebrew/bin:$HOME/.cargo/bin:$PATH"

echo "══════════════════════════════════════════════════"
echo "  📦 Building MusicConvert (Rust Release Pipeline)"
echo "══════════════════════════════════════════════════"

# 1. Build Host Native Release
echo "▶ 1. Building host native release binary..."
cargo build --release --bin MusicConvert
echo "✅ Native binary generated at: target/release/MusicConvert"

# 2. Linux / Synology Cross-Compilation (if targets or cross available)
if rustup target list 2>/dev/null | grep -q 'x86_64-unknown-linux-musl (installed)'; then
    echo "▶ 2. Building Synology static musl binary (x86_64-unknown-linux-musl)..."
    cargo build --release --target x86_64-unknown-linux-musl --bin MusicConvert
    cp target/x86_64-unknown-linux-musl/release/MusicConvert MusicConvert_synology_amd64
    echo "✅ Synology AMD64 binary generated: MusicConvert_synology_amd64"
elif command -v cross >/dev/null 2>&1; then
    echo "▶ 2. Cross-compiling for Linux targets via cross..."
    cross build --release --target x86_64-unknown-linux-musl --bin MusicConvert
    cp target/x86_64-unknown-linux-musl/release/MusicConvert MusicConvert_synology_amd64
fi

echo "🎉 Build complete."
