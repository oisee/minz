#!/usr/bin/env bash
set -euo pipefail
: "${BASE_SHA:?}" "${REPORT_DIR:?}"
mkdir -p "$REPORT_DIR"
export GOLANGCI_LINT_CACHE=${GOLANGCI_LINT_CACHE:-/tmp/minz-golangci-cache}
start=$SECONDS
changed_go=$(git diff --name-only --diff-filter=ACMR "$BASE_SHA" HEAD -- ':(glob)minzc/**/*.go')
if [[ -z $changed_go ]]; then
    printf '{"Issues": []}\n' > "$REPORT_DIR/lint.json"
    printf 'No changed Go source files; Go lint skipped.\n' > "$REPORT_DIR/lint.log"
    status=0
else
    # Lint changed packages only. Running ./pkg/... also type-checks optional
    # graphics packages unrelated to a compiler PR and creates false failures.
    declare -A changed_packages=()
    while IFS= read -r path; do
        relative=${path#minzc/}
        package_dir=${relative%/*}
        changed_packages["./$package_dir/..."]=1
    done <<< "$changed_go"
    mapfile -t lint_targets < <(printf '%s\n' "${!changed_packages[@]}" | LC_ALL=C sort)
    set +e
    (cd minzc && golangci-lint run --new-from-rev="$BASE_SHA" --show-stats=false --output.json.path stdout --output.text.path /dev/null "${lint_targets[@]}") > "$REPORT_DIR/lint.json" 2> "$REPORT_DIR/lint.log"
    status=$?
    set -e
fi
python3 - "$REPORT_DIR" "$status" "$((SECONDS-start))" <<'PY'
import json, os, pathlib, subprocess, sys
root = pathlib.Path(sys.argv[1])
try:
    issues = json.loads((root / 'lint.json').read_text()).get('Issues') or []
except (ValueError, AttributeError):
    issues = []
    print('::warning::golangci-lint produced no JSON; see lint.log')
counts = {}
for issue in issues:
    name = issue['FromLinter']
    counts[name] = counts.get(name, 0) + 1
    pos = issue['Pos']
    text = issue['Text'].replace('%', '%25').replace('\r', '%0D').replace('\n', '%0A')
    print(f'::warning file=minzc/{pos["Filename"]},line={pos["Line"]},col={pos["Column"]}::{name}: {text}')
# gofmt checks whole changed files, including unchanged lines, but never old files.
paths = subprocess.check_output(['git', 'diff', '--name-only', '--diff-filter=ACMR', '-z',
                                 os.environ['BASE_SHA'], 'HEAD', '--', ':(glob)minzc/**/*.go']).split(b'\0')
format_issues = 0
for path in paths:
    if not path:
        continue
    name = os.fsdecode(path)
    result = subprocess.run(['gofmt', '-l', name], capture_output=True, text=True, check=True)
    if result.stdout:
        format_issues += 1
        print(f'::warning file={name},line=1::gofmt: run gofmt on this changed file')
summary = f'### Advisory lint\n\nExit {sys.argv[2]}; wall {sys.argv[3]}s; new issues {len(issues)}: {counts}; gofmt files {format_issues}.\n'
summary += '\n```text\n' + '\n'.join(line[:1024] for line in (root / 'lint.log').read_text().splitlines()[-50:]) + '\n```\n'
if len(summary.encode('utf-8')) > 64 * 1024:
    summary = summary.encode('utf-8')[:64 * 1024].decode('utf-8', errors='ignore') + '\n[Truncated; see artifact.]\n'
(root / 'summary.md').write_text(summary)
print(summary)
with open(os.environ.get('GITHUB_STEP_SUMMARY', os.devnull), 'a') as out:
    out.write(summary)
PY
exit "$status"
