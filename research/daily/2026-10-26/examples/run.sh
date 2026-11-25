#!/bin/sh
# Reproduces the listings and counts quoted in chapters 11-13.
# Usage, from the repository root:  sh research/daily/2026-10-26/examples/run.sh <scratch-dir>
# Builds the current compiler and three older ones (from git archive, into <scratch-dir>)
# and writes every .a80 file the chapters quote into <scratch-dir>.
set -e
S=$1; [ -n "$S" ] || { echo "usage: run.sh <scratch-dir>"; exit 2; }
R=$(git rev-parse --show-toplevel); E=$R/research/daily/2026-10-26/examples
mkdir -p "$S"
(cd "$R/minzc" && go build -o "$S/mz" ./cmd/minzc && go build -o "$S/mza" ./cmd/mza && go build -o "$S/mze" ./cmd/mze)
build_old() {  # $1 = commit, $2 = binary name
  mkdir -p "$S/src-$1"; git -C "$R" archive "$1" minzc | tar -x -C "$S/src-$1"
  (cd "$S/src-$1/minzc" && go build -o "$S/$2" ./cmd/minzc)
}
build_old 3859f45d9^ mz-pre309     # parent of #309
build_old c012b0db1^ mz-pre315     # parent of #315
build_old 6fd07f8a9^ mz-pre317     # parent of #317 (contains #315)
build_old 6fd07f8a9 mz-317         # #317 merge
cd "$S"

echo "== chapter 11: bool in flags (#314)"
cp "$E/11-is-digit.nanz" digits.nanz
./mz digits.nanz --asserts z80 -o digits.a80 2>&1 | grep ASSERTS || true
./mz digits.nanz --no-flag-return -o digits_nofr.a80 >/dev/null
for v in digits digits_nofr; do
  { printf '    ORG $8000\n    LD SP, $FF00\n    CALL count_digits\n    DI\n    HALT\n'; cat $v.a80; } > h_$v.a80
  ./mza h_$v.a80 -o h_$v.bin >/dev/null
  set +e; ./mze h_$v.bin -c > h_$v.run 2>&1; rc=$?; set -e
  echo "$v: $(wc -c < h_$v.bin | tr -d ' ') bytes, $(grep 'Total execution' h_$v.run), exit code (A) $rc"
done

echo "== chapter 12: factorial(7) (#309)"
cp "$E/12-factorial.nanz" fact.nanz
./mz-pre309 fact.nanz --asserts z80 -o /dev/null 2>&1 | grep -E "got|ASSERTS" || true
./mz fact.nanz --asserts z80 --assert-receipt -o /dev/null 2>&1 | grep ASSERTS || true
./mz-pre309 fact.nanz --asserts none -o fact_pre309.a80 >/dev/null
./mz fact.nanz --asserts none -o fact_now.a80 >/dev/null

echo "== chapter 13: ptr_add (#315) and pointer walk (#317)"
cp "$R/research/c89-benchmark/heldout/checksum16.c" "$R/research/c89-benchmark/heldout/find_max.c" .
./mz-pre315 checksum16.c --annotate-tstates -o cs_pre315.a80 >/dev/null
./mz-pre317 checksum16.c --annotate-tstates -o cs_with315.a80 >/dev/null
./mz-pre317 find_max.c --Osize --annotate-tstates -o fm_pre317.a80 >/dev/null
./mz find_max.c --Osize --annotate-tstates -o fm_now.a80 >/dev/null
./mz-317 find_max.c --Osize --annotate-tstates -o fm_317.a80 >/dev/null
grep -v '^; \[trace' fm_317.a80 > a.tmp; grep -v '^; \[trace' fm_now.a80 > b.tmp
cmp -s a.tmp b.tmp && echo "find_max --Osize: #317 merge and current tree emit the same code" || echo "find_max --Osize: differs between #317 merge and current tree"
./mz find_max.c -o fm_now_default.a80 >/dev/null
ls *.a80
