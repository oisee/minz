# Decoding Images from Random Numbers: PRNG Art on Z80

**How GPU brute-force finds seeds that make random number generators draw faces.**

---

## The Idea

A 16-bit LFSR (Linear Feedback Shift Register) generates pseudo-random numbers.
Given a seed, the sequence is deterministic. The sequence looks random — but
if you pick the right seed, the "random" pattern of pixels happens to look
like something recognizable.

The trick: search billions of seeds on GPU until one produces a pattern
that matches a target image. The decoder is trivial — just run the LFSR
and XOR pixels. The encoder is the hard part (brute-force on GPU).

**Decoder: ~50 bytes of Z80 code. Data: 2-8 KB of seeds. Result: recognizable image.**

## The LFSR

```nanz
// 16-bit Galois LFSR — the entire "decompression" engine
fun lfsr16(state: u16) -> u16 {
    let bit: u8 = state % 2        // → AND 1 on Z80 (strength reduced)
    var next: u16 = state / 2      // → SRL H; RR L on Z80
    if bit == 1 { next = next xor 0xB400 }
    if next == 0 { next = 1 }      // avoid degenerate zero state
    return next
}
```

On Z80, this compiles to:

```z80
lfsr16:
    SRL H           ; 8T
    RR  L           ; 8T — 16-bit shift right, bit 0 → CY
    RET NC          ; 5T — skip XOR 50% of the time!
    LD  A, H        ; 4T
    XOR $B4         ; 7T — tap polynomial
    LD  H, A        ; 4T
    RET             ; 10T
; Average: ~35T per step. Period: 65535.
```

**7 instructions. 35T average. That's the entire decoder engine.**

## The Encoding: Cascade AND Masking

One LFSR layer XORs ~50% of cells (random noise). Not useful alone.
The insight: **AND-mask the LFSR output** to reduce density.

```
AND-3:  keep bit only if lower 3 bits = 111    → 12.5% density
AND-4:  keep bit only if lower 4 bits = 1111   →  6.25%
AND-5:  keep bit only if lower 5 bits = 11111  →  3.125%
AND-7:  keep bit only if lower 7 bits all 1    →  0.78%
```

Lower density = more control over which pixels flip.
Stack multiple layers: coarse (AND-3, large blocks) → fine (AND-7, single pixels).

```
Layer 0:  AND-3, blk=4  → coarse "where is the face" (12.5% of cells)
Layer 1:  AND-3, blk=2  → medium detail, 4 quadrants
Layer 2:  AND-4, blk=1  → fine detail, 16 sub-regions
Layer 3:  AND-5..7       → pixel-level correction
```

## The Data Format

Each seed record: 8 bytes.

```
{ seed: u16,     // LFSR starting state
  ox: u8,        // X offset on screen (0, 32, 64, 96)
  oy: u8,        // Y offset (0, 24, 48, 72)
  blk: u8,       // block size (4, 2, or 1 pixel)
  and_n: u8,     // AND mask bits (3, 4, 5, 6, 7)
  warmup: u16 }  // LFSR warmup steps (decorrelate from previous)
```

**1171 seeds × 8 bytes = 9.4 KB** for the full Che Guevara face at 128×96.
Result: 1.2% binary pixel error. Recognizable from across the room.

## The Decoder Algorithm

```
for each seed record:
    1. Initialize LFSR with seed
    2. Advance warmup steps (decorrelate)
    3. Generate 32×24 = 768 random bits via LFSR
    4. AND-mask: keep only bits where (state & mask) == mask
    5. For each set bit: XOR a blk×blk solid block at (ox+bx*blk, oy+by*blk)
```

In Nanz:

```nanz
fun process_seed(seed: u16, and_n: u8, wu: u16, ox: u8, oy: u8, blk: u8) -> void {
    g_state = seed
    warmup(wu)

    var mask: u8 = 1
    var m: u8 = 0
    while m < and_n { mask = mask + mask  m = m + 1 }
    mask = mask - 1     // (1 << and_n) - 1

    fill_buf(mask)      // LFSR → 768-byte buffer
    apply_buf(ox, oy, blk)   // buffer → XOR blocks on screen
}
```

## Progressive Refinement

The cascade applies seeds in order. Early seeds create coarse structure,
later seeds refine details:

```
Step   21:  L_bin ~49%   — first coarse strokes (blk=4, AND-3)
Step  149:  L_bin ~24%   — medium detail visible
Step  213:  L_bin ~13%   — face clearly recognizable
Step  597:  L_bin ~3.5%  — fine details (hair, eyes, beret)
Step 1171:  L_bin ~1.2%  — final quality
```

Each step XORs a sparse mask onto the canvas. XOR is self-inverse:
applying the same mask twice cancels it. This means the decoder can
run any subset of seeds and still produce a valid (if lower quality) image.

## ZX Spectrum Screen Layout

The ZX Spectrum screen is famously non-linear:

```
Address = 0x4000 + (y/64)*2048 + (y%8)*256 + ((y/8)%8)*32 + x/8
Bit     = 7 - (x % 8)
```

Within a character cell (8×8 pixels), consecutive rows are 256 bytes apart
(`INC H` on Z80). This is why the XOR block loop is tight:

```z80
; XOR 8×8 solid block — 8 iterations
xor_block:
    LD  B, 8        ; 8 rows
.loop:
    LD  A, (HL)     ; 7T — read pixel byte
    CPL             ; 4T — XOR 0xFF (complement)
    LD  (HL), A     ; 7T — write back
    INC H           ; 4T — next row (+256 bytes)
    DJNZ .loop      ; 13T — 35T per row, 280T per block
```

In Nanz, the same pattern:

```nanz
fun xor_block(cx: u8, cy: u8) -> void {
    var addr: u16 = 0x4000 + (cy / 8) * 2048 + (cy % 8) * 32 + cx
    for r in 0..8 {
        let p: ^u8 = addr
        p^ = p^ xor 0xFF       // → LD A,(HL); CPL; LD (HL),A
        addr = addr + 256       // → INC H
    }
}
```

`p^ = p^ xor 0xFF` compiles to exactly `LD A,(HL); CPL; LD (HL),A`.
`addr + 256` compiles to `INC H`. **Zero overhead from the language.**

## Size Comparison

| Version | Size | What |
|---------|------|------|
| LFSR-16 engine only | 14 bytes | 7 Z80 instructions |
| Decoder + XOR block | ~120 bytes | complete renderer minus data |
| Hand-optimized ASM (seg0) | 121 bytes | single segment, hardcoded |
| Nanz compiled (seg0) | 382 bytes | same algorithm, compiled |
| Full cascade data | 9.4 KB | 1171 seeds for 1.2% error |
| ZX Spectrum screen | 6.9 KB | 256×192 pixels + attributes |

The decoder is **smaller than the screen it renders**.
With 1171 seeds at 8 bytes each, the total image data (9.4 KB) is
larger than the screen (6.9 KB) — so this is NOT compression.
It's **algorithmic art**: the image exists as a mathematical property
of the LFSR polynomial and seed sequence.

## GPU Search: How Seeds Are Found

The GPU tests millions of seeds per second:

```
For each candidate seed (0..65535):
    1. Run LFSR with seed, generate 768-bit mask
    2. Apply AND-N filter
    3. XOR onto current canvas
    4. Compare with target image (Hamming distance)
    5. If error decreased → keep this seed
    6. Undo XOR (apply same mask again) if rejected
```

On NVIDIA RTX 4060 Ti: ~65536 seeds tested per second per AND-level.
Finding 1171 good seeds takes ~2 hours of GPU time.

The cascade search is greedy: each seed is the locally best improvement.
No backtracking, no global optimization. Yet the result is 1.2% error —
better than most hand-drawn pixel art at this resolution.

## The Philosophical Point

The image of Che Guevara **already exists** in the mathematical structure
of the LFSR polynomial 0xB400. Every possible 128×96 binary image is
reachable through some sequence of seeds. The GPU search doesn't create
the image — it discovers the coordinates that reveal it.

A Z80 running at 3.5 MHz needs about 0.5 seconds to decode all 1171 seeds.
The GPU needed 2 hours to find them. The ratio — 14,400:1 — is the cost
of discovery vs reproduction. Once found, the seeds are eternal:
the same 9.4 KB will produce the same face on any Z80, forever.

## Chapter 2: Carrier-Payload Codec — 3× Fewer Seeds

The cascade approach uses 1171 seeds for 1.2% error. Can we do better?

### The Problem with Flat Cascade

Every seed sprays random points across its entire region. Most points
land on areas that are already correct — wasted work. At AND-7 density
(0.78%), you need hundreds of seeds just to correct a few pixels.

### Carrier-Payload: Two-Level Search

Split each delta into two parts:

**Carrier** (blk=8, AND-3): finds WHERE motion/error is concentrated.
Produces a 32×24 bitmask of "hot zones" — character cells that need work.

**Payload** (blk=4→2→1, AND-4→7): sprays points ONLY inside carrier-activated cells.
The payload LFSR buffer is AND-masked with the carrier bitmask before applying.

```
Traditional:  seed → buffer → XOR entire region
Carrier-Payload:  carrier_seed → hot_mask
                  payload_seed → buffer & hot_mask → XOR masked region
```

### Why It Works

The carrier focuses the payload's "correlation budget" on the ~20% of the screen
that actually needs correction. Instead of wasting 80% of random points on
already-correct areas, every payload point lands where it matters.

**Result: 3× fewer seeds** for the same visual quality.

### Data Format

```json
{
  "type": "cp",
  "cs": 12345,           // carrier seed
  "ps": [45678, 23456],  // payload seeds (applied inside carrier mask)
  "blk": 2,
  "and_n": 5,
  "ox": 32, "oy": 24
}
```

### On Z80

The decoder adds one extra step — AND the payload buffer with the carrier buffer:

```nanz
// Traditional: fill_buf → apply_buf
// CP mode:     fill_carrier → fill_payload → AND together → apply_buf

fun mask_payload(carrier_addr: u16, payload_addr: u16) -> void {
    for i in 0..768 {
        let c: ^u8 = carrier_addr + i
        let p: ^u8 = payload_addr + i
        p^ = p^ & c^    // AND mask: only keep payload where carrier active
    }
}
```

Extra cost: 768 AND operations × ~15T = ~11.5K T-states. Negligible vs the
savings from fewer seeds (each seed = ~50K T-states of LFSR + XOR work).

### Budget Comparison

| Method | Seeds for Che | Data size | Decode time |
|--------|--------------|-----------|-------------|
| Flat cascade | 1171 | 9.4 KB | ~0.5s |
| Carrier-Payload | ~400 | ~3.2 KB | ~0.2s |
| CP for ZX tape | ~400 | ~1.6 KB (4B/seed) | 0.32s load |

For ZX Spectrum tape at 1500 baud: 1.6 KB loads in **0.85 seconds**.
A recognizable face from less than one second of tape audio.

## Chapter 3: Seeds Are Incompressible

A surprising result from entropy analysis:

```
Field      Entropy     Raw size    gzip ratio
seed       ~11.8 bits  16 bits     98-100%
and_n      ~1.5 bits   3 bits      <1%
blk        ~1.2 bits   2 bits      <1%
ox, oy     ~3 bits     8 bits      ~40%
warmup     varies      16 bits     ~60%
```

**Seeds cannot be compressed.** By construction, the GPU search picks seeds
that maximize decorrelation with all previously chosen seeds. This means
the seed sequence has near-maximum entropy — it's already as compact as
random data.

The metadata fields (and_n, blk, ox, oy) compress well because they follow
a structured cascade pattern. But seeds are ~74% of the data, and they're
incompressible.

**Implication: the only "compression" is better algorithms** — fewer seeds
for the same quality. Carrier-Payload achieves this: 3× fewer seeds
by focusing each seed's work on hot zones.

### Optimal Binary Format

```
4 bytes per seed:
  [seed: u16] [flags: u8 = ox_hi:2|oy_hi:2|blk:2|andN:2] [warmup: u8]

1171 seeds × 4 = 4.7 KB
400 CP seeds × 4 = 1.6 KB
```

No entropy coding, no Huffman, no LZ77. The data is already at maximum entropy.
The format is just packed fields. Decodable with zero memory overhead.

## Try It

```bash
# Python reference renderer
python3 -c "
import json
from PIL import Image
with open('z80-optimizer/data/cascade_seeds.json') as f:
    data = json.load(f)
# ... (see cascade_seeds_readme.md for full renderer)
"

# Nanz → Z80
mz fun/che_cascade.nanz -o build/che.a80
mza build/che.a80 -o build/che.bin

# Direct ASM
mza fun/che_optimal.asm -o build/che_asm.bin    # 121 bytes, seg0 only
```

---

*Seeds from z80-optimizer (CUDA brute-force).*
*LFSR-16 polynomial 0xB400, period 65535.*
*Cascade AND-3→7, 6 resolution levels.*
*1171 seeds, 1.2% binary pixel error.*
*Decoder: 7 Z80 instructions.*
