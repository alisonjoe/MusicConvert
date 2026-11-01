#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT_DIR"

export PATH="/opt/homebrew/bin:$HOME/.cargo/bin:$PATH"

echo "Building MusicConvert Rust release binary..."
cargo build --release --bin MusicConvert

echo "Native release binary generated at target/release/MusicConvert"

# If x86_64-unknown-linux-musl target is installed, build static Linux binary
if rustup target list 2>/dev/null | grep -q 'x86_64-unknown-linux-musl (installed)'; then
    echo "Building Synology static musl binary (x86_64-unknown-linux-musl)..."
    cargo build --release --target x86_64-unknown-linux-musl --bin MusicConvert
    cp target/x86_64-unknown-linux-musl/release/MusicConvert MusicConvert_linux_amd64_rust
fi

echo "Done."
