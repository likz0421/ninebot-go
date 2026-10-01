#!/usr/bin/env sh
# build.sh — cross-compile ninecli for all supported platforms.
set -e
cd "$(dirname "$0")"
mkdir -p dist
export CGO_ENABLED=0
export GOPROXY="https://goproxy.cn,direct"

for platform in windows/amd64 windows/arm64 linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
    os="${platform%/*}"
    arch="${platform#*/}"
    ext=""
    [ "$os" = "windows" ] && ext=".exe"
    out="dist/ninecli-${os}-${arch}${ext}"
    echo "building $out"
    GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags "-s -w" -o "$out" .
done

echo "done -> dist/"
