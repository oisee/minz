#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 1 ]; then
    echo "usage: $0 /path/to/mzv" >&2
    exit 2
fi

fixture_dir=$(cd "$(dirname "$0")" && pwd)
vm=$(realpath "$1")
tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT

cp "$fixture_dir/stream3.z3" "$tmp_dir/zork-west-demo.z3"
cp "$fixture_dir/../zork-west-demo/zvm.nanz" "$tmp_dir/zvm.nanz"
mkdir -p "$tmp_dir/text"
cp "$fixture_dir/../../../stdlib/text/print.nanz" "$tmp_dir/text/print.nanz"
"$vm" -H "$tmp_dir/zvm.nanz" > "$tmp_dir/stream3.out"
diff -u "$fixture_dir/stream3.out" "$tmp_dir/stream3.out"

python3 "$fixture_dir/high_memory.py" "$tmp_dir/zork-west-demo.z3"
"$vm" -H "$tmp_dir/zvm.nanz" > "$tmp_dir/high_memory.out"
diff -u "$fixture_dir/high_memory.out" "$tmp_dir/high_memory.out"

cp "$fixture_dir/transcript.z3" "$tmp_dir/zork-west-demo.z3"
"$vm" -H "$tmp_dir/zvm.nanz" > "$tmp_dir/transcript.out"
diff -u "$fixture_dir/transcript.out" "$tmp_dir/transcript.out"
printf 'AB' > "$tmp_dir/transcript.expected"
cmp "$tmp_dir/transcript.expected" "$tmp_dir/zvm-transcript.txt"

cp "$fixture_dir/unicode.z3" "$tmp_dir/zork-west-demo.z3"
rm "$tmp_dir/zvm-transcript.txt"
"$vm" -H "$tmp_dir/zvm.nanz" > "$tmp_dir/unicode.out"
diff -u "$fixture_dir/unicode.out" "$tmp_dir/unicode.out"
printf 'äéœ¿' > "$tmp_dir/unicode.expected"
cmp "$tmp_dir/unicode.expected" "$tmp_dir/zvm-transcript.txt"

cp "$fixture_dir/command_stream.z3" "$tmp_dir/zork-west-demo.z3"
printf 'a\nz\n' | "$vm" -H "$tmp_dir/zvm.nanz" > "$tmp_dir/command_stream.out"
diff -u "$fixture_dir/command_stream.out" "$tmp_dir/command_stream.out"
printf 'a\n' > "$tmp_dir/command_stream.expected"
cmp "$tmp_dir/command_stream.expected" "$tmp_dir/zvm-commands.txt"

cp "$fixture_dir/unicode_input.z3" "$tmp_dir/zork-west-demo.z3"
rm "$tmp_dir/zvm-commands.txt"
printf 'Äéœ\n' | "$vm" -H "$tmp_dir/zvm.nanz" > "$tmp_dir/unicode_input.out"
diff -u "$fixture_dir/unicode_input.out" "$tmp_dir/unicode_input.out"
printf 'Äéœ\n' > "$tmp_dir/unicode_input.expected"
cmp "$tmp_dir/unicode_input.expected" "$tmp_dir/zvm-commands.txt"

cp "$fixture_dir/save_restore.z3" "$tmp_dir/zork-west-demo.z3"
printf 's\nr\nq\n' | "$vm" -H "$tmp_dir/zvm.nanz" > "$tmp_dir/save_restore.out"
diff -u "$fixture_dir/save_restore.out" "$tmp_dir/save_restore.out"
test -s "$tmp_dir/zvm-save.dat"

# A save from a different release must fail before replacing dynamic memory.
python3 - "$tmp_dir/zork-west-demo.z3" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
story = bytearray(path.read_bytes())
story[3] ^= 1
path.write_bytes(story)
PY
printf 'r\nq\n' | "$vm" -H "$tmp_dir/zvm.nanz" > "$tmp_dir/mismatch.out"
grep -Fq 'restore failed' "$tmp_dir/mismatch.out"
if grep -Fq 'saved 1' "$tmp_dir/mismatch.out"; then
    echo 'restored a mismatched story' >&2
    exit 1
fi
