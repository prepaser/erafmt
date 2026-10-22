#!/usr/bin/env bash
set -euo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")"
version=$(tr -d '\r\n' < VERSION)
mkdir -p dist
outputs=()

for target in linux/amd64 linux/arm64 windows/amd64 windows/arm64 darwin/amd64 darwin/arm64; do
    goos=${target%/*}
    goarch=${target#*/}
    ext=
    if [[ $goos == windows ]]; then
        ext=.exe
    fi
    output="erafmt_${version}_${goos}_${goarch}${ext}"
    printf 'Building %s\n' "$target"
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
        go build -trimpath -buildvcs=false -ldflags="-s -w" -o "dist/$output" .
    outputs+=("$output")
done

(cd dist && sha256sum "${outputs[@]}" > SHA256SUMS)
