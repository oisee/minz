#!/usr/bin/env bash
# Rebuild the showcase with the current compiler and assembler.
set -u

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
export GOCACHE=${GOCACHE:-"$tmp/go-cache"}

(cd "$root/minzc" && go build -o "$tmp/mz" ./cmd/minzc && go build -o "$tmp/mza" ./cmd/mza) || exit 1

checked=0
blocked=0
errors=0

known_assembler_blocker() {
    case "$1" in
        fun/raymarcher.nanz|fun/vectors.nanz|fun/irc_client.nanz|fun/fun/che_intro.nanz) return 0 ;;
        *) return 1 ;;
    esac
}

while IFS= read -r source; do
    checked=$((checked + 1))
    id=${source//\//_}
    asm="$tmp/$id.a80"
    if ! "$tmp/mz" "$root/$source" --asserts mir2 -o "$asm" >"$tmp/$id.compile.log" 2>&1; then
        echo "COMPILE FAIL  $source"
        tail -n 8 "$tmp/$id.compile.log"
        errors=$((errors + 1))
        continue
    fi
    if "$tmp/mza" "$asm" -o "$tmp/$id.bin" >"$tmp/$id.assemble.log" 2>&1; then
        if known_assembler_blocker "$source"; then
            echo "FIXED         $source (remove from known_assembler_blocker)"
            errors=$((errors + 1))
        else
            echo "OK            $source"
        fi
    elif known_assembler_blocker "$source"; then
        echo "KNOWN BLOCKER $source"
        blocked=$((blocked + 1))
    else
        echo "ASSEMBLE FAIL $source"
        tail -n 8 "$tmp/$id.assemble.log"
        errors=$((errors + 1))
    fi
done < <(cd "$root" && find fun -path 'fun/archive' -prune -o -type f \( -name '*.nanz' -o -name '*.frl' \) -print | sort)

while IFS= read -r source; do
    checked=$((checked + 1))
    id=${source//\//_}
    if (cd "$root/$(dirname "$source")" && "$tmp/mza" "$(basename "$source")" -o "$tmp/$id.bin") >"$tmp/$id.assemble.log" 2>&1; then
        echo "OK            $source"
    else
        echo "ASSEMBLE FAIL $source"
        tail -n 8 "$tmp/$id.assemble.log"
        errors=$((errors + 1))
    fi
done < <(cd "$root" && find fun -path 'fun/archive' -prune -o -type f -name '*.asm' -print | sort)

echo "$checked sources checked; $blocked known assembler blockers; $errors regressions"
test "$errors" -eq 0
