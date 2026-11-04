#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT_DIR"

GOCACHE_DIR="${GOCACHE:-/private/tmp/musicconvert-gocache}"

echo "Building Synology linux/amd64..."
env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOCACHE="$GOCACHE_DIR" \
	go build -trimpath -ldflags="-s -w" -o MusicConvert_synology_amd64 .

echo "Building Synology linux/arm64..."
env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 GOCACHE="$GOCACHE_DIR" \
	go build -trimpath -ldflags="-s -w" -o MusicConvert_synology_arm64 .

echo "Building Synology linux/armv7..."
env CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 GOCACHE="$GOCACHE_DIR" \
	go build -trimpath -ldflags="-s -w" -o MusicConvert_synology_armv7 .

echo "Done:"
file MusicConvert_synology_amd64 MusicConvert_synology_arm64 MusicConvert_synology_armv7
