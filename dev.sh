#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

config="${GLIMPSE_CONFIG:-.dev/config.json}"
binary=".dev/glimpse-dev"
poll_interval="${GLIMPSE_DEV_POLL_INTERVAL:-1}"
server_pid=""

watch_files() {
  {
    find . -maxdepth 1 \( -name '*.go' -o -name 'go.mod' -o -name 'go.sum' \) -type f -print
    find assets templates -type f -print 2>/dev/null || true
    if [[ -f "$config" ]]; then
      printf '%s\n' "$config"
    fi
  } | LC_ALL=C sort
}

snapshot() {
  watch_files | while IFS= read -r file; do
    if [[ -f "$file" ]]; then
      cksum "$file" || true
    else
      printf 'missing %s\n' "$file"
    fi
  done | cksum | awk '{print $1 ":" $2}'
}

stop_server() {
  if [[ -n "${server_pid:-}" ]] && kill -0 "$server_pid" 2>/dev/null; then
    kill "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  server_pid=""
}

cleanup() {
  stop_server
}

rebuild_and_restart() {
  mkdir -p "$(dirname "$binary")"
  echo "[dev] building..."
  if ! go build -o "$binary" .; then
    echo "[dev] build failed; keeping the current server running"
    return
  fi

  stop_server
  echo "[dev] starting with $config"
  GLIMPSE_DEV=1 "$binary" -dev -config "$config" &
  server_pid=$!
}

trap cleanup EXIT
trap 'exit 0' INT TERM

last_snapshot="$(snapshot)"
rebuild_and_restart

while true; do
  sleep "$poll_interval"

  if [[ -n "${server_pid:-}" ]] && ! kill -0 "$server_pid" 2>/dev/null; then
    wait "$server_pid" 2>/dev/null || true
    echo "[dev] server stopped"
    server_pid=""
  fi

  next_snapshot="$(snapshot)"
  if [[ "$next_snapshot" != "$last_snapshot" ]]; then
    last_snapshot="$next_snapshot"
    echo "[dev] change detected"
    rebuild_and_restart
  fi
done
