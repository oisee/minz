#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -lt 2 ] || [ "$#" -gt 3 ]; then
    echo "usage: $0 /path/to/mzv /path/to/pinned-mit-zork1.z3 [/path/to/dfrotz]" >&2
    exit 2
fi

demo_dir=$(cd "$(dirname "$0")" && pwd)
vm=$(realpath "$1")
story=$(realpath "$2")
expected_sha=66e54935b47bf9d76e05bc97ba42844ae8e7294a0625bc08c9cac206f4b68b51
actual_sha=$(sha256sum "$story" | cut -d ' ' -f 1)
if [ "$actual_sha" != "$expected_sha" ]; then
    echo "Zork I story SHA-256 mismatch: $actual_sha" >&2
    exit 1
fi

tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT
cp "$story" "$tmp_dir/zork-west-demo.z3"
cp "$demo_dir/zvm.nanz" "$tmp_dir/zvm.nanz"
printf 'look\nopen mailbox\ntake leaflet\nread leaflet\ninventory\nquit\ny\n' | "$vm" -H "$tmp_dir/zvm.nanz" > "$tmp_dir/output"
python3 - "$tmp_dir/output" <<'PY'
from pathlib import Path
import sys

output = Path(sys.argv[1]).read_text()
for expected in (
    'ZORK I: The Great Underground Empire',
    'West of House',
    'There is a small mailbox here.',
    'Opening the small mailbox reveals a leaflet.',
    'ZORK is a game of adventure, danger, and low cunning.',
    'You are carrying:\n  A leaflet',
    'Your score is 0 (total of 350 points), in 5 moves.',
    'This gives you the rank of Beginner.',
    'Do you wish to leave the game? (Y is affirmative):',
):
    if expected not in output:
        raise SystemExit(f'Zork I output missing: {expected!r}\n{output}')
if output.count('There is a small mailbox here.') != 2:
    raise SystemExit('Zork I LOOK did not describe West of House twice')
for failure in ('Z3 error',):
    if failure in output:
        raise SystemExit(f'Zork I output contains: {failure}')
print('MIT Zork I v3: six-command gameplay transcript passed (>64 KiB story)')
PY

if [ "$#" -eq 3 ]; then
    frotz=$(realpath "$3")
    printf 'look\nopen mailbox\ntake leaflet\nread leaflet\ninventory\nquit\ny\n' | "$frotz" -m -w 255 "$story" > "$tmp_dir/frotz-output"
    python3 - "$tmp_dir/output" "$tmp_dir/frotz-output" <<'PY'
from pathlib import Path
import sys

def game_lines(path):
    return [line.lstrip('> ').rstrip() for line in Path(path).read_text().splitlines()
            if line.strip() and 'Score: 0        Moves:' not in line]

if game_lines(sys.argv[1]) != game_lines(sys.argv[2]):
    raise SystemExit('MIT Zork I gameplay text differs from Frotz')
print('MIT Zork I v3: gameplay text matches Frotz (excluding status lines)')
PY
fi

printf 'save\nrestore\nquit\ny\n' | "$vm" -H "$tmp_dir/zvm.nanz" > "$tmp_dir/save-output"
python3 - "$tmp_dir/save-output" <<'PY'
from pathlib import Path
import sys

output = Path(sys.argv[1]).read_text()
if output.count('>Ok.') != 2:
    raise SystemExit(f'Zork I save/restore did not resume twice at save: {output}')
if 'Your score is 0 (total of 350 points), in 0 moves.' not in output:
    raise SystemExit('Zork I restore did not return to initial turn count')
print('MIT Zork I v3: save/restore passed')
PY
test -s "$tmp_dir/zvm-save.dat"
