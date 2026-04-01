#!/usr/bin/env python3
"""Generate seeds.bin: 64 × (seed_lo, seed_hi, points) = 192 bytes.
GPU-found optimal seeds for Che Guevara ZX Spectrum LFSR intro."""
import struct, sys

SEEDS = [
    (0xC6A1, 128), (0xDE22, 126), (0xA455, 124), (0x6CCA, 122),
    (0x6CCA, 120), (0x6CCA, 118), (0x6CCA, 116), (0x6CCA, 114),
    (0x43AD, 112), (0x2D28, 110), (0x2D28, 108), (0x2D28, 106),
    (0xCE7A, 104), (0x0C20, 102), (0xF88E, 100), (0x40B0,  98),
    (0x6370,  48), (0x3498,  47), (0x70C8,  46), (0xE447,  45),
    (0xD595,  44), (0x1840,  43), (0x1F56,  42), (0x5E92,  41),
    (0x3449,  40), (0x0BE0,  39), (0x2968,  38), (0x83C6,  37),
    (0x894A,  36), (0x5C3A,  35), (0xB662,  34), (0xD337,  33),
    (0x744F,  24), (0x99F1,  23), (0xC176,  23), (0x060D,  22),
    (0x56A7,  22), (0x3E4D,  21), (0x2FCD,  21), (0x4AB5,  20),
    (0x9475,  20), (0x3A0F,  19), (0x5E9C,  19), (0x2B1C,  18),
    (0x6EF7,  18), (0x0FD2,  17), (0xAED2,  17), (0x5EB5,  16),
    (0x7CE5,  16), (0x4BE5,  15), (0x0BEA,  15), (0x33EC,  14),
    (0x7D0D,  14), (0x0613,  13), (0x13F0,  13), (0x96CE,  12),
    (0x4CCE,  12), (0x091E,  11), (0x2A8D,  11), (0x5364,  10),
    (0xB494,  10), (0x15C5,   9), (0x0176,   9), (0x02EA,   8),
]

assert len(SEEDS) == 64
sys.stdout.buffer.write(b''.join(struct.pack('<HB', s, p) for s, p in SEEDS))
