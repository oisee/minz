#!/usr/bin/env python3
"""
gen_anim_bin.py — Convert animation JSON → ANMZ binary for Z80 anim_player.

Binary format:
  u8[4]  "ANMZ"
  u16 LE n_frames
  u8     fps
  u8     _pad
  × n_frames × 4 bytes: [u16 seed_count LE, u8 frame_type(0=kf,1=dt), u8 _pad]
  × total_seeds × 7 bytes: [u16 seed LE, u8 ox,oy,blk,and_n,warmup]

Usage:
    python3 gen_anim_bin.py input.json output.bin [--fps 5]
    python3 gen_anim_bin.py input.json output.bin --bake anim_player.asm -o baked.asm
"""
import json, struct, sys, argparse, subprocess, tempfile, os
from pathlib import Path

MARKER = "; ANIM_DATA_BEGIN"
COLS   = 16


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


def load_frames(data, fps_override=None):
    """Returns (fps, [(ftype, [seed_entry, ...]), ...])"""
    seeds_raw = [normalise(r) for r in data.get('seeds', []) if isinstance(r, dict)]
    fps = fps_override or data.get('fps', 5)

    if data.get('type') in ('animation_flat', 'animation'):
        frame_starts = data.get('frame_starts', [0])
        frame_types  = data.get('frame_types',  [])
        n_frames     = data.get('num_frames', len(frame_starts))
        frames = []
        for fi in range(n_frames):
            start = frame_starts[fi] if fi < len(frame_starts) else 0
            end   = frame_starts[fi+1] if fi+1 < len(frame_starts) else len(seeds_raw)
            ftype = 0 if (fi < len(frame_types) and frame_types[fi] == 'kf') else 1
            frames.append((ftype, seeds_raw[start:end]))
        return fps, frames
    else:
        # Static: single keyframe
        return fps, [(0, seeds_raw)]


def frames_to_bin(fps, frames):
    n_frames = len(frames)
    out = b'ANMZ'
    out += struct.pack('<HBB', n_frames, int(fps), 0)
    seeds_all = []
    for ftype, seeds in frames:
        out += struct.pack('<HBB', len(seeds), ftype, 0)
        seeds_all.extend(seeds)
    for e in seeds_all:
        out += struct.pack('<HBBBBB',
            e['seed']   & 0xFFFF,
            e['ox']     & 0xFF,
            e['oy']     & 0xFF,
            e['blk']    & 0xFF,
            e['and_n']  & 0xFF,
            e['warmup'] & 0xFF)
    return out, n_frames, len(seeds_all)


def bin_to_defb(data):
    lines = []
    for i in range(0, len(data), COLS):
        chunk = data[i:i+COLS]
        lines.append("        DEFB    " + ",".join(f"${b:02X}" for b in chunk))
    return "\n".join(lines)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('input',  help='animation seeds JSON')
    ap.add_argument('output', nargs='?', default=None, help='output .bin (or use --bake)')
    ap.add_argument('--fps',  type=float, default=None, help='override fps')
    ap.add_argument('--bake', metavar='TEMPLATE', default=None,
                    help='bake into ASM template (replaces ; ANIM_DATA_BEGIN)')
    ap.add_argument('-o', '--asm-out', default=None,
                    help='output baked .asm (default: print) or .bin output path')
    args = ap.parse_args()

    data = json.loads(Path(args.input).read_text())
    fps, frames = load_frames(data, args.fps)
    raw, n_frames, n_seeds = frames_to_bin(fps, frames)

    total_seeds = sum(len(s) for _, s in frames)
    print(f'{n_frames} frames · {total_seeds} seeds · {len(raw)} bytes · {fps} fps',
          file=sys.stderr)

    if args.bake:
        template = Path(args.bake).read_text()
        if MARKER not in template:
            sys.exit(f"error: marker '{MARKER}' not found in {args.bake}")
        defb = bin_to_defb(raw)
        baked = template.replace(MARKER, MARKER + "\n" + defb)

        out_asm = args.asm_out or (Path(args.bake).stem + '_baked.asm')
        if out_asm == '-':
            print(baked)
        else:
            Path(out_asm).write_text(baked)
            print(f'wrote {out_asm}', file=sys.stderr)

        # Assemble
        bin_out = Path(out_asm).with_suffix('.bin')
        result = subprocess.run(['mza', out_asm, '-o', str(bin_out)],
                                capture_output=True, text=True)
        if result.returncode != 0:
            print(result.stderr, file=sys.stderr)
            sys.exit(result.returncode)
        print(f'assembled → {bin_out}', file=sys.stderr)
    else:
        out = args.asm_out or args.output or (Path(args.input).stem + '.anmz.bin')
        Path(out).write_bytes(raw)
        print(f'wrote {out}', file=sys.stderr)


if __name__ == '__main__':
    main()
