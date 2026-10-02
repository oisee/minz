#!/usr/bin/env bash
set -euo pipefail
: "${BASE_SHA:?}" "${CI_BIN:?}"
mkdir -p "$CI_BIN"
base_root=$(mktemp -d "$CI_BIN/base.XXXXXX")
trap 'rm -rf "$base_root"' EXIT
# Archive rather than worktree: no mutation of the main checkout's Git metadata.
git archive "$BASE_SHA" | gzip > "$CI_BIN/base.tar.gz"
tar -xzf "$CI_BIN/base.tar.gz" -C "$base_root"
export GOFLAGS=-buildvcs=false
if [[ ! -x $CI_BIN/baseline-mz ]]; then
  (cd "$base_root/minzc" && go build -o "$CI_BIN/baseline-mz" ./cmd/minzc)
fi
(cd minzc && go build -o "$CI_BIN/candidate-mz" ./cmd/minzc)
# The source archive contains only the base SHA's tracked files. Downstream
# jobs create their own Git index for inventory; no object database is uploaded.
