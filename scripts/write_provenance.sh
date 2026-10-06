#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: $0 OUTPUT_FILE REPOSITORY_ROOT" >&2
  exit 2
fi

output=$1
repo=$2
mkdir -p "$(dirname "$output")"
{
  echo "captured_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "host=$(hostname)"
  uname -a
  if git -C "$repo" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    echo "git_commit=$(git -C "$repo" rev-parse HEAD)"
    echo "git_dirty=$(test -n "$(git -C "$repo" status --porcelain)" && echo true || echo false)"
  else
    echo "git_commit=unavailable"
    echo "git_dirty=unavailable"
  fi
  go version
  go env GOTOOLCHAIN GOOS GOARCH GOPATH GOMODCACHE
  sha256sum "$repo/attack.go" "$repo/go.mod" "$repo/go.sum"
} > "$output"
