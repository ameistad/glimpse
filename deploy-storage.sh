#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

host="${GLIMPSE_DEPLOY_HOST:-andreas@storage}"
remote_binary="${GLIMPSE_REMOTE_BINARY:-~/glimpse-server}"
remote_deploy_cmd="${GLIMPSE_REMOTE_DEPLOY_CMD:-./deploy-glimpse.sh}"
binary="${GLIMPSE_DEPLOY_BINARY:-glimpse-linux}"
cc="${CC:-x86_64-linux-musl-gcc}"

if ! command -v "$cc" >/dev/null 2>&1; then
  echo "Missing compiler: $cc" >&2
  echo "Install musl-cross or set CC to a Linux-capable CGO compiler." >&2
  exit 1
fi

echo "[deploy] running tests"
go test ./...

echo "[deploy] building linux amd64 binary with $cc"
CGO_ENABLED=1 CC="$cc" GOOS=linux GOARCH=amd64 \
  go build -ldflags="-linkmode external -extldflags '-static'" \
  -o "$binary" .

echo "[deploy] uploading $binary to $host:$remote_binary"
scp "$binary" "$host:$remote_binary"

echo "[deploy] running $remote_deploy_cmd on $host"
ssh -t "$host" "$remote_deploy_cmd"
