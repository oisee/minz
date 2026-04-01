#!/usr/bin/env python3
"""
gen_seeds_bin.py — Convert seeds JSON → compact binary for Z80 replay.

Binary format:
  [n_seeds u16 LE]
  per entry × n_seeds:
    [seed u16 LE][ox u8][oy u8][blk u8][and_n u8][warmup u8]   = 7 bytes

Usage:
    python3 gen_seeds_bin.py input.json output.bin
    python3 gen_seeds_bin.py input.json output.bin --frame 0    # animation: one frame
"""
import json, struct, sys, argparse
from pathlib import Path


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


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('input')
    ap.add_argument('output')
    ap.add_argument('--frame', type=int, default=None,
                    help='animation: extract single frame (0-based)')
    args = ap.parse_args()

    data = json.loads(Path(args.input).read_text())
    seeds = [normalise(r) for r in data.get('seeds', []) if isinstance(r, dict)]

    if args.frame is not None and data.get('type') in ('animation_flat', 'animation'):
        fs = data.get('frame_starts', [])
        fi = args.frame
        start = fs[fi] if fi < len(fs) else 0
        end   = fs[fi + 1] if fi + 1 < len(fs) else len(seeds)
        seeds = seeds[start:end]
        print(f'frame {fi}: {len(seeds)} seeds')

    out = struct.pack('<H', len(seeds))
    for e in seeds:
        out += struct.pack('<HBBBBB',
            e['seed'] & 0xFFFF,
            e['ox']   & 0xFF,
            e['oy']   & 0xFF,
            e['blk']  & 0xFF,
            e['and_n']& 0xFF,
            e['warmup']& 0xFF)

    Path(args.output).write_bytes(out)
    print(f'wrote {args.output}: {len(seeds)} seeds × 7 bytes = {len(out)} bytes')


if __name__ == '__main__':
    main()
