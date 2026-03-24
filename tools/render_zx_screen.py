#!/usr/bin/env python3
"""Render ZX Spectrum screen from mze profile memory snapshot to PNG."""
import json
import sys

try:
    from PIL import Image
except ImportError:
    print("pip install Pillow")
    sys.exit(1)

# ZX Spectrum colors (RGB)
ZX_COLORS = [
    (0, 0, 0),       # 0 black
    (0, 0, 215),     # 1 blue
    (215, 0, 0),     # 2 red
    (215, 0, 215),   # 3 magenta
    (0, 215, 0),     # 4 green
    (0, 215, 215),   # 5 cyan
    (215, 215, 0),   # 6 yellow
    (215, 215, 215), # 7 white
]

ZX_BRIGHT_COLORS = [
    (0, 0, 0),       # 0 black
    (0, 0, 255),     # 1 blue
    (255, 0, 0),     # 2 red
    (255, 0, 255),   # 3 magenta
    (0, 255, 0),     # 4 green
    (0, 255, 255),   # 5 cyan
    (255, 255, 0),   # 6 yellow
    (255, 255, 255), # 7 white
]

def render(profile_path, output_path):
    with open(profile_path) as f:
        data = json.load(f)

    snap = data.get('mem_snapshot', {})

    # Read screen memory (6144 bytes at $4000-$57FF)
    screen = [0] * 6144
    for addr_hex, val in snap.items():
        addr = int(addr_hex, 16)
        v = int(val) if isinstance(val, (int, float)) else int(str(val), 16)
        if 0x4000 <= addr < 0x5800:
            screen[addr - 0x4000] = v

    # Read attribute memory (768 bytes at $5800-$5AFF)
    attrs = [0x38] * 768  # default: white ink on black paper
    for addr_hex, val in snap.items():
        addr = int(addr_hex, 16)
        v = int(val) if isinstance(val, (int, float)) else int(str(val), 16)
        if 0x5800 <= addr < 0x5B00:
            attrs[addr - 0x5800] = v

    # Render 256x192 image
    img = Image.new('RGB', (256, 192))
    pixels = img.load()

    for py in range(192):
        for px in range(256):
            # ZX Spectrum screen address calculation
            # The screen memory is interleaved:
            # Address = 010TTSSS LLLCCCCC
            # TT = third (0-2), SSS = scan line within char (0-7)
            # LLL = character row within third (0-7), CCCCC = column (0-31)
            char_row = py // 8
            scan_line = py % 8
            char_col = px // 8
            pixel_bit = 7 - (px % 8)

            # Screen address
            third = char_row // 8
            row_in_third = char_row % 8
            addr = (third << 11) | (scan_line << 8) | (row_in_third << 5) | char_col

            byte = screen[addr]
            pixel_set = (byte >> pixel_bit) & 1

            # Attribute
            attr_idx = char_row * 32 + char_col
            attr = attrs[attr_idx]

            ink = attr & 0x07
            paper = (attr >> 3) & 0x07
            bright = (attr >> 6) & 0x01

            palette = ZX_BRIGHT_COLORS if bright else ZX_COLORS

            if pixel_set:
                pixels[px, py] = palette[ink]
            else:
                pixels[px, py] = palette[paper]

    # Scale up 2x for visibility
    img = img.resize((512, 384), Image.NEAREST)
    img.save(output_path)
    print(f"Saved {output_path} (512x384)")

if __name__ == '__main__':
    render(sys.argv[1] if len(sys.argv) > 1 else '/tmp/sap_zx_profile.json',
           sys.argv[2] if len(sys.argv) > 2 else '/tmp/sap_zx_screen.png')
