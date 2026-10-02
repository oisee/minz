#!/usr/bin/env bash
set -euo pipefail
# Run from the repository root after fetching full history.
base=$(git rev-parse "${BASE_REV:-HEAD~1}^{commit}")
controls=false
if [[ ${NIGHTLY:-false} == true ]]; then
  controls=true
else
  # Includes deleted/renamed paths. Frontends parse assertions; HIR owns receipts;
  # the WASM/LLVM runners execute sandbox assertions.
  changed=$(git diff --no-renames --name-only "$base" HEAD)
  while IFS= read -r path; do
    case "$path" in
      scripts/* | minzc/cmd/minzc/* | minzc/pkg/pipeline/* | \
      minzc/pkg/nanz/* | minzc/pkg/c89/* | minzc/pkg/pascal/* | \
      minzc/pkg/plm/* | minzc/pkg/abap/* | minzc/pkg/frill/* | \
      minzc/pkg/lanz/* | minzc/pkg/lizp/* | minzc/pkg/hir/hir.go | \
      minzc/pkg/hir/assert*.go | minzc/pkg/mir2wasm/runner.go | \
      minzc/pkg/mir2llvm/runner.go)
        controls=true ;;
    esac
  done <<< "$changed"
fi
printf 'base=%s\ncontrols=%s\n' "$base" "$controls" | tee -a "${GITHUB_OUTPUT:-/dev/null}"
