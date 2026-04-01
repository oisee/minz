#!/usr/bin/env python3
"""
render_seeds.py — LFSR-16 AND-cascade XOR renderer.

Reads seeds JSON (from z80-optimizer GPU search) → PNG / animated GIF.
Same algorithm as z80-optimizer/docs/renderer.html.

Usage:
    python3 render_seeds.py seeds.json              # → seeds.png
    python3 render_seeds.py seeds.json -o out.png   # static image
    python3 render_seeds.py seeds.json --gif        # → seeds.gif (animation)
    python3 render_seeds.py seeds.json --gif --fps 5 --scale 4
"""
import json, sys, argparse
from pathlib import Path

W, H = 128, 96   # canvas pixels


# ── Core algorithm ──────────────────────────────────────────────────────────

def lfsr16(s: int) -> int:
    """Single LFSR-16 step: right-shift, XOR 0xB400 if LSB was set."""
    return (s >> 1) ^ (0xB400 if s & 1 else 0)


def make_buf(seed: int, warmup: int, and_n: int) -> bytearray:
    """Generate 768-byte block buffer (32×24 grid, each 0 or 1)."""
    s = seed if seed != 0 else 1
    for _ in range(warmup):
        s = lfsr16(s)
    buf = bytearray(768)
    for i in range(768):
        acc = 1
        for _ in range(and_n):
            s = lfsr16(s)
            acc &= s & 1
        buf[i] = acc
    return buf


def apply_buf(canvas: bytearray, buf: bytearray, ox: int, oy: int, blk: int):
    """XOR buffer blocks onto canvas."""
    for by in range(24):
        for bx in range(32):
            if not buf[by * 32 + bx]:
                continue
            for dy in range(blk):
                for dx in range(blk):
                    x = ox + bx * blk + dx
                    y = oy + by * blk + dy
                    if 0 <= x < W and 0 <= y < H:
                        canvas[y * W + x] ^= 1


def render_frame(entries) -> bytearray:
    """Render a list of seed entries onto a fresh canvas, return 128×96 bytes."""
    canvas = bytearray(W * H)
    for e in entries:
        buf = make_buf(e['seed'], e.get('warmup', e.get('w', 0)),
                       e.get('and_n', e.get('n', 3)))
        apply_buf(canvas, buf, e.get('ox', 0), e.get('oy', 0),
                  e.get('blk', e.get('b', 4)))
    return canvas


# ── Normalise JSON ──────────────────────────────────────────────────────────

def normalise(r: dict) -> dict:
    """Expand compact keys s/b/n/w/f → seed/blk/and_n/warmup/frame."""
    return {
        'seed':   r.get('seed', r.get('s', 0)),
        'ox':     r.get('ox', 0),
        'oy':     r.get('oy', 0),
        'blk':    r.get('blk', r.get('b', 4)),
        'and_n':  r.get('and_n', r.get('n', 3)),
        'warmup': r.get('warmup', r.get('w', 0)),
        'frame':  r.get('frame', r.get('f', 0)),
    }


def load_json(path: str):
    with open(path) as f:
        return json.load(f)


# ── PNG output (no PIL dependency) ─────────────────────────────────────────

def canvas_to_png_bytes(canvas: bytearray, scale: int = 4) -> bytes:
    """Encode 128×96 binary canvas as PNG (pure Python, no deps)."""
    import struct, zlib
    sw, sh = W * scale, H * scale
    rows = []
    for y in range(H):
        row = bytearray()
        for x in range(W):
            v = 255 if canvas[y * W + x] else 0
            row += bytes([v] * scale)
        row_bytes = bytes(row)
        rows += [b'\x00' + row_bytes] * scale   # filter byte 0 = None
    idat = zlib.compress(b''.join(rows), 9)

    def chunk(tag, data):
        c = struct.pack('>I', len(data)) + tag + data
        return c + struct.pack('>I', zlib.crc32(tag + data) & 0xFFFFFFFF)

    return (b'\x89PNG\r\n\x1a\n'
            + chunk(b'IHDR', struct.pack('>IIBBBBB', sw, sh, 8, 0, 0, 0, 0))
            + chunk(b'IDAT', idat)
            + chunk(b'IEND', b''))


def save_png(canvas: bytearray, path: str, scale: int = 4):
    Path(path).write_bytes(canvas_to_png_bytes(canvas, scale))
    print(f'saved {path}')


# ── GIF output (no PIL dependency) ─────────────────────────────────────────

def save_gif(frames: list, path: str, fps: float = 5, scale: int = 4):
    """Write animated GIF (1-bit palette, no deps)."""
    try:
        from PIL import Image
        _save_gif_pil(frames, path, fps, scale)
    except ImportError:
        _save_gif_raw(frames, path, fps, scale)


def _save_gif_pil(frames, path, fps, scale):
    from PIL import Image
    imgs = []
    for canvas in frames:
        sw, sh = W * scale, H * scale
        img = Image.new('L', (sw, sh))
        for y in range(H):
            for x in range(W):
                v = 255 if canvas[y * W + x] else 0
                for dy in range(scale):
                    for dx in range(scale):
                        img.putpixel((x * scale + dx, y * scale + dy), v)
        imgs.append(img.convert('P'))
    delay = max(1, int(100 / fps))
    imgs[0].save(path, save_all=True, append_images=imgs[1:],
                 loop=0, duration=delay)
    print(f'saved {path} ({len(frames)} frames @ {fps:.1f}fps)')


def _save_gif_raw(frames, path, fps, scale):
    """Minimal GIF89a writer (no deps, 1-bit palette)."""
    import struct
    sw, sh = W * scale, H * scale
    delay_cs = max(1, int(100 / fps))

    def lzw_compress(data, min_code_size=2):
        import io
        cs = max(2, min_code_size)
        clear = 1 << cs
        eoi = clear + 1
        table = {bytes([i]): i for i in range(clear)}
        code_size = cs + 1
        out = io.BytesIO()
        buf, bit_pos = 0, 0
        def emit(code):
            nonlocal buf, bit_pos
            buf |= code << bit_pos
            bit_pos += code_size
            while bit_pos >= 8:
                out.write(bytes([buf & 0xFF]))
                buf >>= 8; bit_pos -= 8
        emit(clear)
        buf_seq = b''
        next_code = eoi + 1
        for byte in data:
            candidate = buf_seq + bytes([byte])
            if candidate in table:
                buf_seq = candidate
            else:
                emit(table[buf_seq])
                table[candidate] = next_code
                next_code += 1
                if next_code > (1 << code_size) and code_size < 12:
                    code_size += 1
                buf_seq = bytes([byte])
        emit(table[buf_seq])
        emit(eoi)
        if bit_pos: out.write(bytes([buf & 0xFF]))
        raw = out.getvalue()
        result = bytes([cs])
        for i in range(0, len(raw), 255):
            chunk = raw[i:i+255]
            result += bytes([len(chunk)]) + chunk
        return result + b'\x00'

    def frame_bytes(canvas):
        pixels = bytearray()
        for y in range(H):
            for _ in range(scale):
                for x in range(W):
                    v = 1 if canvas[y * W + x] else 0
                    pixels += bytes([v] * scale)
        return bytes(pixels)

    out = bytearray()
    # Header
    out += b'GIF89a'
    out += struct.pack('<HH', sw, sh)
    out += bytes([0xF0, 0, 0])           # global CT 2 colours
    out += bytes([0, 0, 0, 255, 255, 255])  # black, white

    # Loop
    out += b'\x21\xFF\x0B' + b'NETSCAPE2.0' + b'\x03\x01\x00\x00\x00'

    for canvas in frames:
        # GCE
        out += b'\x21\xF9\x04\x00'
        out += struct.pack('<H', delay_cs)
        out += b'\x00\x00'
        # Image
        out += b'\x2C'
        out += struct.pack('<HHHHB', 0, 0, sw, sh, 0)
        out += lzw_compress(frame_bytes(canvas))

    out += b'\x3B'
    Path(path).write_bytes(out)
    print(f'saved {path} ({len(frames)} frames @ {fps:.1f}fps, pure python)')


# ── Main ────────────────────────────────────────────────────────────────────

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('input', help='seeds JSON file')
    ap.add_argument('-o', '--output', help='output file (default: input stem + .png/.gif)')
    ap.add_argument('--gif', action='store_true', help='output animated GIF')
    ap.add_argument('--fps', type=float, default=5.0, help='GIF fps (default 5)')
    ap.add_argument('--scale', type=int, default=4, help='pixel scale factor (default 4)')
    ap.add_argument('--frame', type=int, default=None, help='single frame to render (animation only)')
    args = ap.parse_args()

    data = load_json(args.input)
    stem = Path(args.input).stem
    is_anim = data.get('type') in ('animation_flat', 'animation')

    raw_seeds = data.get('seeds', [])
    seeds = [normalise(r) for r in raw_seeds if isinstance(r, dict)]

    if is_anim and args.gif:
        # Animated GIF: accumulate canvas across frames
        frame_starts = data.get('frame_starts', [])
        frame_types  = data.get('frame_types', [])
        num_frames   = data.get('num_frames', len(frame_starts))
        fps          = args.fps or data.get('fps', 5)

        frames = []
        canvas = bytearray(W * H)

        for fi in range(num_frames):
            start = frame_starts[fi] if fi < len(frame_starts) else 0
            end   = frame_starts[fi + 1] if fi + 1 < len(frame_starts) else len(seeds)
            ftype = frame_types[fi] if fi < len(frame_types) else 'dt'

            if ftype == 'kf':
                canvas = bytearray(W * H)

            for e in seeds[start:end]:
                buf = make_buf(e['seed'], e['warmup'], e['and_n'])
                apply_buf(canvas, buf, e['ox'], e['oy'], e['blk'])

            frames.append(bytes(canvas))

        out = args.output or f'{stem}.gif'
        save_gif(frames, out, fps, args.scale)

    elif is_anim and args.frame is not None:
        # Single frame from animation
        frame_starts = data.get('frame_starts', [])
        fi = args.frame
        start = frame_starts[fi] if fi < len(frame_starts) else 0
        end   = frame_starts[fi + 1] if fi + 1 < len(frame_starts) else len(seeds)
        canvas = render_frame(seeds[start:end])
        out = args.output or f'{stem}_f{fi:03d}.png'
        save_png(canvas, out, args.scale)

    else:
        # Static image (all seeds in one canvas)
        canvas = render_frame(seeds)
        out = args.output or f'{stem}.png'
        save_png(canvas, out, args.scale)


if __name__ == '__main__':
    main()
