#!/usr/bin/env bash
set -euo pipefail
# Arguments: authoritative repository, preserved measured tokenizer directory.
REPO_ROOT=${1:?repository root required}
BASELINE_DIR=${2:?measured baseline artifact directory required}
EVIDENCE_DIR=$(cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(cd -- "$REPO_ROOT" && pwd)
BASELINE_DIR=$(cd -- "$BASELINE_DIR" && pwd)
python3 - "$REPO_ROOT" "$EVIDENCE_DIR" <<'PY'
import json,sys
from pathlib import Path
repo,pack=map(Path,sys.argv[1:]);target=repo/'minzc/pkg/pipeline/profile_workload.go'
json.dump({'Replace':{str(target):str(pack/'profile_workload_overlay.go.txt')}},open(pack/'overlay.json','w'))
PY
cd "$REPO_ROOT/minzc"
MINZ_PC_BASELINE_DIR="$BASELINE_DIR" MINZ_PC_AUDIT_OUTPUT="$EVIDENCE_DIR/reproduced-pc-records.json" go run -overlay="$EVIDENCE_DIR/overlay.json" "$EVIDENCE_DIR/audit.go"
