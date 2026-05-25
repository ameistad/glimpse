#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

host="${GLIMPSE_DEPLOY_HOST:-andreas@storage}"
remote_binary="${GLIMPSE_REMOTE_BINARY:-~/glimpse-server}"
remote_deploy_cmd="${GLIMPSE_REMOTE_DEPLOY_CMD:-./deploy-glimpse.sh}"
binary="${GLIMPSE_DEPLOY_BINARY:-glimpse-linux}"
cc="${CC:-x86_64-linux-musl-gcc}"
full_rescan=false

usage() {
  cat <<EOF
Usage: $0 [--full-rescan|--reset-db]

Deploys the binary and preserves the existing media database by default.

Options:
  --full-rescan, --reset-db  Delete the remote database before restart so Glimpse rebuilds it.
  -h, --help                 Show this help.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --full-rescan|--reset-db)
      full_rescan=true
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

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

remote_args=()
if [[ "$full_rescan" == true ]]; then
  echo "[deploy] full rescan requested; remote deploy will delete the database"
  remote_args+=(--reset-db)
else
  echo "[deploy] preserving database; startup scan will skip unchanged media"
fi

remote_cmd="$remote_deploy_cmd"
if ((${#remote_args[@]})); then
  printf -v quoted_args " %q" "${remote_args[@]}"
  remote_cmd+="$quoted_args"
fi

echo "[deploy] running $remote_cmd on $host"
ssh -t "$host" "$remote_cmd"
