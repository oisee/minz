# Animated LFSR-16 AND-cascade player — pseudocode

## Binary format at ANIM_ADDR ($CC00)

```
u8[4]   "ANMZ"           magic
u16 LE  n_frames
u8      fps               1–50  (ZX Spectrum: 50Hz vsync via HALT)
u8      _pad
× n_frames × 4 bytes:
    u16 LE  seed_count    seeds in this frame
    u8      frame_type    0=kf (clear+render)  1=dt (XOR delta)
    u8      _pad
× total_seeds × 7 bytes:
    u16 LE  seed
    u8      ox, oy, blk, and_n, warmup
```

Seeds data starts immediately after the frame table:
`seeds_start = ANIM_ADDR + 8 + n_frames × 4`

## Algorithm

```
halts_per_frame = 50 / fps      // e.g. fps=5 → 10 HALTs per frame

seeds_base = ANIM_ADDR + 8 + n_frames × 4

loop forever:
    seeds_ptr = seeds_base      // rewind for each loop iteration

    for f = 0 .. n_frames-1:
        if frame_type[f] == KF:
            memset(screen $4000, 0, 6144)   // clear pixel RAM

        for i = 0 .. seed_count[f]-1:
            s  = seeds_ptr.seed
            ox = seeds_ptr.ox
            oy = seeds_ptr.oy
            blk    = seeds_ptr.blk
            and_n  = seeds_ptr.and_n
            warmup = seeds_ptr.warmup
            seeds_ptr += 7

            make_buf(s, warmup, and_n)      // fill BUF[768]
            apply_buf(ox, oy, blk)          // XOR blocks onto screen

        repeat halts_per_frame:
            HALT                            // wait for 50Hz interrupt
```

## Notes

- `HALT` on ZX Spectrum stalls until the next maskable interrupt (50 Hz PAL).
  So halts_per_frame = 50/fps gives accurate frame timing.
- `dt` frames XOR their seeds onto whatever is already on screen —
  to undo frame N before applying frame N+1 is NOT needed; the GPU
  encodes deltas so applying frame N+1's seeds produces the correct image.
- For seamless looping the last frame should leave the screen in the
  same state as before the first `kf` frame (or the first frame is `kf`).
