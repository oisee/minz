#!/usr/bin/env python3
"""
bake_replay.py — Bake seeds JSON into replay.asm and assemble.

Replaces the SEEDS_DATA_BEGIN marker with DEFB lines, writes a
self-contained .asm, then calls mza to produce a .bin.

Usage:
    python3 bake_replay.py seeds.json [-o replay_out.bin] [--frame N]
    python3 bake_replay.py seeds.json --asm-only    # just print the baked .asm
"""
import json, struct, sys, argparse, subprocess, tempfile, os
from pathlib import Path

TEMPLATE = Path(__file__).with_name("replay.asm")
MARKER   = "; SEEDS_DATA_BEGIN"
COLS     = 16   # bytes per DEFB line


def normalise(r):
    return {
        'seed':   r.get('seed', r.get('s', 0)),
        'ox':     r.get('ox', 0),
        'oy':     r.get('oy', 0),
        'blk':    r.get('blk', r.get('b', 4)),
        'and_n':  r.get('and_n', r.get('n', 3)),
        'warmup': r.get('warmup', r.get('w', 0)),
        'frame':  r.get('frame', r.get('f', 0)),
    }


def seeds_to_bin(seeds):
    out = struct.pack('<H', len(seeds))
    for e in seeds:
        out += struct.pack('<HBBBBB',
            e['seed']   & 0xFFFF,
            e['ox']     & 0xFF,
            e['oy']     & 0xFF,
            e['blk']    & 0xFF,
            e['and_n']  & 0xFF,
            e['warmup'] & 0xFF)
    return out


def bin_to_defb(data):
    lines = []
    for i in range(0, len(data), COLS):
        chunk = data[i:i+COLS]
        lines.append("        DEFB    " + ",".join(f"${b:02X}" for b in chunk))
    return "\n".join(lines)


def bake(json_path, frame=None):
    data   = json.loads(Path(json_path).read_text())
    seeds  = [normalise(r) for r in data.get('seeds', []) if isinstance(r, dict)]

    if frame is not None and data.get('type') in ('animation_flat', 'animation'):
        fs    = data.get('frame_starts', [])
        start = fs[frame] if frame < len(fs) else 0
        end   = fs[frame+1] if frame+1 < len(fs) else len(seeds)
        seeds = seeds[start:end]
        print(f"frame {frame}: {len(seeds)} seeds", file=sys.stderr)

    raw   = seeds_to_bin(seeds)
    defb  = bin_to_defb(raw)

    template = TEMPLATE.read_text()
    if MARKER not in template:
        sys.exit(f"error: marker '{MARKER}' not found in {TEMPLATE}")

    return template.replace(MARKER, MARKER + "\n" + defb), len(seeds), len(raw)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('input', help='seeds JSON file')
    ap.add_argument('-o', '--output', default=None, help='output .bin (default: replay.bin)')
    ap.add_argument('--frame', type=int, default=None, help='animation frame index')
    ap.add_argument('--asm-only', action='store_true', help='print baked .asm, do not assemble')
    args = ap.parse_args()

    asm_src, n_seeds, n_bytes = bake(args.input, args.frame)

    if args.asm_only:
        print(asm_src)
        return

    out_bin = args.output or "replay.bin"

    with tempfile.NamedTemporaryFile(suffix='.asm', mode='w', delete=False) as f:
        f.write(asm_src)
        tmp = f.name

    try:
        result = subprocess.run(
            ["mza", tmp, "-o", out_bin],
            capture_output=True, text=True
        )
        if result.returncode != 0:
            print(result.stdout, file=sys.stderr)
            print(result.stderr, file=sys.stderr)
            sys.exit(result.returncode)
        print(f"ok: {n_seeds} seeds × 7 bytes = {n_bytes} bytes → {out_bin}")
    finally:
        os.unlink(tmp)


if __name__ == '__main__':
    main()
