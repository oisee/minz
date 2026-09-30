#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -lt 2 ] || [ "$#" -gt 3 ]; then
    echo "usage: $0 /path/to/mzv /path/to/pinned-czech.z3 [/path/to/dfrotz]" >&2
    exit 2
fi

demo_dir=$(cd "$(dirname "$0")" && pwd)
vm=$(realpath "$1")
story=$(realpath "$2")
expected_sha=6d628af7ab4779224cb87923cce3764e3ad3ab8634758f409b638ba20f91d283
actual_sha=$(sha256sum "$story" | cut -d ' ' -f 1)
if [ "$actual_sha" != "$expected_sha" ]; then
    echo "CZECH story SHA-256 mismatch: $actual_sha" >&2
    exit 1
fi

tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT
cp "$story" "$tmp_dir/zork-west-demo.z3"
cp "$demo_dir/zvm.nanz" "$tmp_dir/zvm.nanz"
"$vm" -H "$tmp_dir/zvm.nanz" > "$tmp_dir/output"
python3 - "$tmp_dir/output" <<'PY'
from pathlib import Path
import sys

output = Path(sys.argv[1]).read_text()
for expected in (
    'Jumps [2]:',
    'Variables [32]:',
    'Arithmetic ops [71]:',
    'Logical ops [115]:',
    'Memory [129]:',
    'Subroutines [137]:',
    'Objects [148]:',
    'Indirect Opcodes [227]:',
    'Misc [345]:',
    'print_addr (Hello.): Hello.',
    'A long string that Inform will put in high memory',
    "I love 'xyzzy'  I love 'xyzzy'",
    'Test Object #1Test Object #2',
    'Performed 368 tests.\nPassed: 349, Failed: 0, Print tests: 19',
    "Didn't crash: hooray!\nLast test: quit!",
):
    if expected not in output:
        raise SystemExit(f'CZECH output missing: {expected!r}\n{output}')
for failure in ('ERROR [', 'Z3 error'):
    if failure in output:
        raise SystemExit(f'CZECH output contains: {failure}')
print('CZECH v3: 349/349 assertions passed; 19 print cases reached')
PY

if [ "$#" -eq 3 ]; then
    frotz=$(realpath "$3")
    "$frotz" -m -w 255 "$story" < /dev/null > "$tmp_dir/frotz-output"
    python3 - "$tmp_dir/output" "$tmp_dir/frotz-output" <<'PY'
from pathlib import Path
import sys

def print_lines(path):
    output = Path(path).read_text()
    section = output[output.index('Print opcodes [350]:'):]
    return [line for line in section.splitlines() if line.strip()]

mzv, frotz = map(print_lines, sys.argv[1:])
if mzv != frotz:
    raise SystemExit('CZECH print block differs from Frotz beyond blank-line layout')
output = Path(sys.argv[1]).read_text()
if '[360] new_line:\n\nThere should be an empty line above this line.' not in output:
    raise SystemExit('CZECH new_line did not produce its expected blank line')
print('CZECH v3: 19 print cases match Frotz text; blank-line layout differs')
PY
fi
