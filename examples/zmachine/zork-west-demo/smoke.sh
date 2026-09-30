#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 1 ]; then
    echo "usage: $0 /path/to/mzv" >&2
    exit 2
fi

demo_dir=$(cd "$(dirname "$0")" && pwd)
actual=$(mktemp)
trap 'rm -f "$actual"' EXIT
"$1" -H "$demo_dir/zvm.nanz" < "$demo_dir/transcript.in" > "$actual"
diff -u "$demo_dir/transcript.out" "$actual"
