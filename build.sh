#!/bin/bash
set -euo pipefail

# Script to Build the binary and injecting the version
# ./build.sh          # normal describe
# ./build.sh dirty    # describe with --dirty

DESCRIBE_ARGS=(--tags --always)

if [[ "${1:-}" == "dirty" ]]; then
  DESCRIBE_ARGS+=(--dirty)
elif [[ $# -gt 0 ]]; then
  echo "Usage: $0 [dirty]" >&2
  exit 1
fi

VERSION=$(git describe "${DESCRIBE_ARGS[@]}")
COMMIT=$(git rev-parse HEAD)
BUILD_TIME=$(date -u +%Y-%m-%d)

go build \
  -ldflags "-X github.com/rohmanhakim/docs-crawler/internal/build.Version=$VERSION -X github.com/rohmanhakim/docs-crawler/internal/build.Commit=$COMMIT -X github.com/rohmanhakim/docs-crawler/internal/build.BuildTime=$BUILD_TIME" \
  -o ./cmd/crawler ./cmd/crawler
