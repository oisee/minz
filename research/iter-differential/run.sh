#!/bin/bash
# Differential run of the iterator-chain regression set (see README.md).
# usage: run.sh <tree-A> <tree-B> <outdir>
#   Builds mz and tier3 from each tree's minzc/, then for every program in
#   progs/ records: mz exit status, sha256 of the .a80, and the tier3 line
#   (raw MIR2 VM / optimised MIR2 VM / Z80 assert results). Prints a table of
#   the programs whose record differs between A and B, and a summary.
set -u
here=$(cd "$(dirname "$0")" && pwd)
A=$1; B=$2; out=$3
mkdir -p "$out/A" "$out/B"
for side in A B; do
  tree=$A; [ $side = B ] && tree=$B
  go -C "$tree/minzc" build -o "$out/$side/mz" ./cmd/minzc || exit 1
  go -C "$tree/minzc" build -o "$out/$side/tier3" "$here/tier3/main.go" || exit 1
done
n=0; d=0
printf '%s\t%s\t%s\t%s\t%s\n' program rc_A rc_B a80_same tier3_A_vs_B > "$out/table.tsv"
for f in "$here"/progs/*; do
  name=$(basename "$f"); n=$((n+1))
  for side in A B; do
    (cd "$out/$side" && ./mz "$f" -o "$out/$side/$name.a80" > "$out/$side/$name.log" 2>&1; echo $? > "$out/$side/$name.rc")
    [ -f "$out/$side/$name.a80" ] && shasum -a 256 < "$out/$side/$name.a80" | cut -c1-16 > "$out/$side/$name.sha" || echo none > "$out/$side/$name.sha"
    "$out/$side/tier3" "$f" 2>/dev/null | grep -v '^\[' | grep -v '^  line' | sed "s#^$f: ##" > "$out/$side/$name.tier"
  done
  same=yes; cmp -s "$out/A/$name.sha" "$out/B/$name.sha" || same=no
  ta=$(cat "$out/A/$name.tier"); tb=$(cat "$out/B/$name.tier")
  if [ "$(cat $out/A/$name.rc)" != "$(cat $out/B/$name.rc)" ] || [ $same = no ] || [ "$ta" != "$tb" ]; then
    d=$((d+1))
    printf '%s\t%s\t%s\t%s\tA: %s | B: %s\n' "$name" "$(cat $out/A/$name.rc)" "$(cat $out/B/$name.rc)" $same "${ta:0:160}" "${tb:0:160}" >> "$out/table.tsv"
  fi
done
cat "$out/table.tsv"
echo "programs=$n differing=$d"
