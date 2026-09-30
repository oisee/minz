#!/usr/bin/env bash
set -euo pipefail

here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
zilf=${ZILF:-zilf}
zapf=${ZAPF:-zapf}
out=${1:-"$here/zork-west-demo.z3"}
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT

cp -- "$here/zork-west-demo.zil" "$work/"
(
  cd -- "$work"
  "$zilf" zork-west-demo.zil
  "$zapf" zork-west-demo.zap zork-west-demo-fixed.z3 -r 1 -s 000001
)
cp -- "$work/zork-west-demo-fixed.z3" "$out"
sha256sum "$out"
