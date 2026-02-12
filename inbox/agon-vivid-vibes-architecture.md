# How Vivid Vibes Works — Full Demo Architecture

*From the agon-vivid-vibes project, for reimplementation reference.*

This letter explains the complete architecture of the Vivid Vibes demo for Agon Light 2. It is a self-contained MOS binary that plays two pre-computed 3D animations (spinning cube + torus) with bytebeat music, using VDP buffered commands for 60fps playback over the UART link.

## High-Level Pipeline

```
Python (offline)           eZ80 (runtime)              VDP (ESP32)
─────────────────          ────────────────            ───────────
gen_cube.py/gen_torus.py   decompress LZSS             store buffers
  → VDU frame bytes        reconstruct VDU frames      in 8MB PSRAM
  → transpose columns      upload as VDP buffers ──→   │
  → LZSS compress          compute bytebeat audio      │
  → .bin blob              upload audio samples ────→   │
                           main loop:                   │
                             call buffer ID ────────→   execute VDU
                             swap ──────────────────→   flip backbuf
                             wait VSYNC                 draw to screen
```

## 1. The Binary Layout

The MOS binary (`vivid.bin`) is built with the agondev C toolchain. Three binary blobs are embedded via assembly `.incbin` directives:

```asm
_setup_data:      .incbin "cube_setup.bin"      ; 11 bytes
_cube_compressed:  .incbin "cube_compressed.bin"  ; ~6.6KB
_torus_compressed: .incbin "torus_compressed.bin" ; ~35KB
```

In C these are declared as `extern uint8_t[]` with matching `_end` labels, so the size is `end - start`.

## 2. Setup Blob

The setup blob is a raw VDU byte sequence sent to switch into graphics mode:

```
Byte sequence: [22, 136, 23, 0, 0xC0, 0, 23, 1, 0, 12, 16]

22, 136         → VDU 22, mode  — set MODE 136 (= mode 8 + 128 for double buffering)
                  Mode 8 = 320x240, 64 colours (2-2-2 RGB)
                  +128 = double-buffered (draw to back buffer, swap on command)
23, 0, 0xC0, 0 → VDU 23, 0, &C0, 0  — switch to pixel coordinates
                  (default is BBC Micro logical coords 1280x1024, origin bottom-left)
                  Pixel mode: origin top-left, Y down, pixel units matching mode resolution
23, 1, 0        → VDU 23, 1, 0  — disable text cursor
12              → VDU 12  — CLS (clear text)
16              → VDU 16  — CLG (clear graphics)
```

This is sent as raw bytes via `mos_puts()` — the MOS function that writes bytes to the VDP UART.

## 3. Compressed Frame Data Format

Each effect (cube, torus) is stored as an LZSS-compressed blob. The on-disk format is:

```
[decompressed_size: u24-LE]     ← 3 bytes
[LZSS compressed data...]       ← rest of file
```

After decompression, the data is in **slot-major columnar layout**:

```
[num_frames: u16-LE]
[max_tris: u8]                  ← max triangles in any single frame
[tri_counts: num_frames bytes]  ← actual triangle count per frame
[colors:  max_tris * num_frames bytes]  ← GCOL colour index per tri
[x1_lo:   max_tris * num_frames bytes]  ← vertex 1 X low byte
[x1_hi:   max_tris * num_frames bytes]  ← vertex 1 X high byte
[y1:      max_tris * num_frames bytes]  ← vertex 1 Y (single byte, 0-239)
[x2_lo:   max_tris * num_frames bytes]
[x2_hi:   max_tris * num_frames bytes]
[y2:      max_tris * num_frames bytes]
[x3_lo:   max_tris * num_frames bytes]
[x3_hi:   max_tris * num_frames bytes]
[y3:      max_tris * num_frames bytes]
```

There are **10 columns** total. Each column has `max_tris * num_frames` bytes.

**Slot-major ordering**: within each column, data is arranged as:
```
slot0_frame0, slot0_frame1, ..., slot0_frameN-1,
slot1_frame0, slot1_frame1, ..., slot1_frameN-1,
...
```

So `column[slot * num_frames + frame]` gives the value for triangle `slot` in `frame`. Frames with fewer triangles than `max_tris` have their extra slots padded with zeros.

**Why this layout?** The same logical triangle slot changes slowly over time (a vertex only moves a few pixels per frame), so consecutive bytes in each column are very similar. LZSS compresses this much better than frame-major layout. The cube achieves ~6.6KB compressed from ~27KB raw VDU data.

## 4. LZSS Decompression

The LZSS format uses a sliding window of 4096 bytes:

```
Input stream: [flag_byte] [token0] [token1] ... [token7] [flag_byte] ...

Flag byte: 8 bits, LSB first. For each bit (0..7):
  bit = 0 → literal: copy next byte to output
  bit = 1 → match:   read 2 bytes encoding a back-reference
    byte0: offset_lo (low 8 bits)
    byte1: (offset_hi << 4) | (length - 3)
    offset = ((byte1 & 0xF0) << 4) | byte0 + 1    → range 1..4096
    length = (byte1 & 0x0F) + 3                     → range 3..18
    Copy `length` bytes from `output[pos - offset]`
```

The decompressor is ~30 lines of C. Here's the core loop:

```c
while (si < src_len && di < dst_size) {
    uint8_t flags = src[si++];
    for (uint8_t bit = 0; bit < 8; bit++) {
        if (flags & (1 << bit)) {
            // Match: decode 12-bit offset + 4-bit length
            uint8_t b0 = src[si++], b1 = src[si++];
            uint16_t offset = ((uint16_t)(b1 & 0xF0) << 4) | b0;
            offset += 1;
            uint8_t length = (b1 & 0x0F) + 3;
            uint24_t copy_from = di - offset;
            for (uint8_t j = 0; j < length && di < dst_size; j++)
                dst[di++] = dst[copy_from + j];
        } else {
            dst[di++] = src[si++];  // Literal
        }
    }
}
```

## 5. Frame Upload — VDP Buffered Commands

After decompression, the player reconstructs VDU drawing commands for each frame and uploads them as **VDP buffers**. This is the key optimization — frames are stored in the VDP's 8MB PSRAM and replayed with a single 6-byte command.

### Reconstructing a frame

For frame `f` with `ntris` triangles:

```
VDU 16                              ← CLG (clear graphics) — 1 byte

For each triangle slot s = 0..ntris-1:
  idx = s * num_frames + f          ← slot-major index into columns

  VDU 18, 0, colors[idx]            ← GCOL 0, colour — set fill colour (3 bytes)

  VDU 25, 4, x1_lo, x1_hi, y1, 0   ← MOVE to vertex 1 (6 bytes)
  VDU 25, 4, x2_lo, x2_hi, y2, 0   ← MOVE to vertex 2 (6 bytes)
  VDU 25, 85, x3_lo, x3_hi, y3, 0  ← PLOT 85 = filled triangle (6 bytes)
```

Each triangle is 21 bytes of VDU data (3 + 6 + 6 + 6). A typical cube frame has ~10 triangles = ~211 bytes.

### Uploading to a VDP buffer

Each frame's VDU payload is wrapped in a buffer-write command:

```
VDU 23, 0, &A0, id_lo, id_hi, 0, len_lo, len_hi, <payload...>
                                 ^command 0 = write
```

- `id`: buffer ID (u16). Cube uses IDs 1..N, torus uses IDs 1000..1000+M.
- `len`: payload length (u16-LE)
- The 8-byte header + payload is sent as one `mos_puts()` call.

### Playing a buffer

To play back a frame, send:

```
VDU 23, 0, &A0, id_lo, id_hi, 1    ← command 1 = call/execute buffer (6 bytes)
VDU 23, 0, &C3                      ← swap double-buffer (3 bytes)
<wait for VSYNC>
```

That's **9 bytes per frame** instead of ~200-1500 bytes streamed. The eZ80-to-VDP UART link is the bottleneck, so this is a ~100x bandwidth reduction.

## 6. The Playback Loop

```c
for (;;) {
    for (uint8_t song = 0; song < NUM_SONGS; song++) {
        play_song(-(song + 1));                      // start bytebeat
        if (play_effect(1, cube_nframes, 2)) break;  // 2 cycles of cube
        if (play_effect(1000, torus_nframes, 2)) break; // 2 cycles of torus
    }
}
```

Each `play_effect()` call loops through buffer IDs, sending call+swap+waitvblank per frame. The Space key exits.

## 7. Bytebeat Audio

Two songs are computed at runtime on the eZ80 and uploaded as 8-bit signed PCM samples to the VDP audio system.

### Song 1: JS-256 Engine

A base-36 melody string defines 32 note slots:

```
"99C9E9GECCGCJCGC77B7C7EC5595C5CB"
```

Each character maps to a chromatic semitone (0='0'..9='9', A=10..Z=35). A frequency LUT maps semitone → `2^(n/12) / 2.67 * 128`:

```c
static const uint8_t js_freq[20] = {
    48, 51, 54, 57, 60, 64, 68, 72, 76, 81,
    85, 90, 96, 102, 108, 114, 121, 128, 136, 144
};
```

The voice function computes a sawtooth with amplitude envelope:
```c
uint8_t saw = (t * js_freq[note]) >> 7;  // sawtooth from freq LUT
uint8_t sub = (r >> 6) & 127;            // sub-oscillator
if (!((saw + sub) & 128)) return 0;      // gate
uint16_t env = 8192 - (r & 8191);        // linear decay envelope
return env >> 6;                          // scale to 0..128
```

Four echoes are summed with halving amplitude (delay = 12288 samples):
```c
val = voice(t, r) + voice(t, r-12288)/2 + voice(t, r-24576)/4 + voice(t, r-36864)/8;
```

The time variable `r` is derived from `t` as `r = t * 213 / 256` (≈ t/1.205).

### Song 2: BPM=160 Arpeggiator

An 8-step pitch sequence with XOR rhythm gate:

```c
uint24_t tf = t * 11;                        // tempo scaling
uint16_t pitch = bpm_pitch[(tf >> 14) & 7];  // 8-step arpeggio (8.8 fixed)
uint8_t saw = (t * pitch) >> 8;              // main sawtooth
uint16_t denom = (half_tf) | ((half_tf>>5) ^ (half_tf>>6));  // XOR gate
uint24_t pulse = (saw << 8) / denom;         // gated pulse
uint8_t bass = (t * bpm_pitch2[(tf>>18)&3]) >> 8;  // bass sub-osc
result = ((pulse & 1) ? 128 : 0) + (bass >> 1);    // combine
```

### Audio Upload

Samples are written to a 128KB buffer (`decomp_buf` reused after frame decompression), then uploaded:

```c
vdp_audio_load_sample(-1, 65536, decomp_buf);  // slot -1 = user sample 1
vdp_audio_set_sample_repeat_start(-1, 0);
vdp_audio_set_sample_repeat_length(-1, 65536);
vdp_audio_sample_rate(0, 8000);                  // 8kHz playback
```

Negative sample IDs are user-defined samples. Channel 0 is used for playback:
```c
vdp_audio_set_waveform(0, sample_id);  // assign sample to channel
vdp_audio_play_note(0, 127, 0, -1);    // volume=127, freq=0 (sample rate), duration=forever
```

## 8. VDP Drawing Primitives Reference

All drawing happens through VDU byte sequences sent over UART:

| VDU Sequence | Meaning |
|---|---|
| `22, mode` | Set screen mode |
| `23, 0, 0xC0, 0` | Switch to pixel coordinates |
| `23, 0, 0xC3` | Swap double-buffer |
| `23, 1, 0` | Cursor off |
| `12` | CLS (clear text area) |
| `16` | CLG (clear graphics area) |
| `18, 0, colour` | GCOL 0, colour — set graphics fill colour |
| `25, 4, x_lo, x_hi, y_lo, y_hi` | MOVE to (x,y) — sets graphics cursor |
| `25, 85, x_lo, x_hi, y_lo, y_hi` | PLOT 85 — filled triangle using last 3 points |
| `25, 69, x_lo, x_hi, y_lo, y_hi` | PLOT 69 — plot single pixel |
| `25, 5, x_lo, x_hi, y_lo, y_hi` | DRAW line to (x,y) |
| `25, 101, x_lo, x_hi, y_lo, y_hi` | PLOT 101 — filled rectangle (corner to corner) |
| `23, 0, 0xA0, id_lo, id_hi, 0, len_lo, len_hi, ...` | Write data to VDP buffer |
| `23, 0, 0xA0, id_lo, id_hi, 1` | Call/execute VDP buffer |
| `23, 0, 0xA0, 0xFF, 0xFF, 2` | Clear all VDP buffers |

Coordinates are 16-bit signed little-endian. In pixel mode, (0,0) is top-left, X increases right, Y increases down.

**Colour mapping (Mode 8):** 64 colours using 2-2-2 RGB encoding:
```
index = (R << 4) | (G << 2) | B
where R, G, B are each 0..3
```

## 9. Reimplementation Guide

To reimplement this in another language targeting Agon:

1. **LZSS decompressor** — ~30 lines, the only algorithm needed. Flag byte + 8 tokens, literal vs 12+4 bit match.

2. **Frame data parser** — read the columnar header, then reconstruct VDU sequences per frame from the 10 columns.

3. **VDP buffer upload** — wrap each frame's VDU bytes in the buffer-write command and send over UART.

4. **Playback loop** — for each frame: send 6-byte call command + 3-byte swap + wait for VSYNC interrupt.

5. **Bytebeat audio** (optional) — compute 64K samples using integer math, upload via VDP audio API.

The offline Python pipeline (compression, transposition) doesn't need reimplementation — the `.bin` blobs are pre-built and just need to be embedded or loaded from SD card.

The minimum viable reimplementation is: LZSS decompress → parse columns → upload buffers → call+swap loop. That's maybe 200 lines of code in any language with UART/VDP access.

## 10. Memory Budget

```
decomp_buf:  128KB — shared between decompression and audio computation
vdu_buf:     1.5KB — scratch for building VDU upload commands
Compressed blobs: ~42KB total (embedded in binary)
Decompressed data: cube ~27KB + torus ~103KB = ~130KB (stored sequentially in decomp_buf... actually exceeds 128KB for torus, uses separate region)
VDP PSRAM:   ~4MB usable for buffers (frames live here after upload)
```

The eZ80 has 512KB of RAM at 0x040000-0x0BFFFF, so this fits comfortably.
