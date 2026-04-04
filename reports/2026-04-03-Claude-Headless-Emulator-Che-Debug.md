# Headless Emulator Debug: che_cascade Black Frames

**Date:** 2026-04-03
**Scope:** Read-only investigation, /home/alice/dev/minz only

---

## 1. mze vs mzx-headless: Which Tool?

**mzx-headless is the right tool.** mze is a pure CPU emulator — no ULA, no
screen memory interpretation, no PNG output. mzx-headless (`main_headless.go`,
371 LOC) has full ZX Spectrum 48K emulation with ULA, VRAM, and `saveScreenPNG()`.

Build: `go build -tags mzx_headless -o mzx-headless ./cmd/mzx`

## 2. Why che_cascade Stays Black

**Root cause: attributes never initialized.** Two independent problems combine:

### Problem A: Missing attribute initialization

che_cascade.nanz never writes to the attribute area (0x5800-0x5AFF). The
program calls `xor_pixel()` which XORs bitmap bytes at 0x4000-0x57FF, but
attributes at 0x5800 remain at their default: `0x00`.

Attribute byte `0x00` = ink 0 (black) on paper 0 (black), bright 0.
Every pixel is drawn **black on black** — invisible.

The `saveScreenPNG()` function (`main_headless.go:281-319`) correctly reads
attributes via `ReadScreen(0x1800 + row*32 + col)`. It's doing the right thing —
the data is just all zeroes.

### Problem B: Preloading white attrs at 0x5800 from the command line doesn't work

The `--load data@0x5800` approach writes to RAM[5] (the screen page) which IS
correct. **However**, the 768 bytes of attributes need to be loaded BEFORE the
program runs, and the program must not clear them. In che_cascade.nanz there's
no clear of the attr area, so preloading SHOULD work — but the `--load` flag
loads raw binary file content. You'd need a 768-byte file of `0x38` bytes
(white paper, black ink) or `0x47` (white ink, black paper, bright).

The most likely failure: the preloaded file was 768 bytes of `0xFF` (all bits set),
which gives bright white ink on bright white paper with flash — still invisible
if the bitmap XORs aren't showing contrast.

**The correct attribute value for white-on-black:** `0x07` (ink=7=white, paper=0=black)
or `0x38` (ink=0=black, paper=7=white).

### Correct invocation:

```bash
# Create 768-byte attr file: ink=7 (white), paper=0 (black)
python3 -c "import sys; sys.stdout.buffer.write(b'\x07'*768)" > /tmp/attrs.bin

# Run with preloaded attrs
mzx-headless --model 48k \
  --load /tmp/attrs.bin@0x5800 \
  --load build/che_cas.bin@0x8000 \
  --set "PC=8000,SP=FFFF,DI,IM=1" \
  --frames 500 \
  --dump-keyframes /tmp/che_kf
```

The order matters: load attrs FIRST, then the binary (which loads at 0x8000,
not touching 0x5800).

## 3. Is the Screenshot Save Path Bug Real?

**Partially.** Two issues:

### 3a. Silent failure on missing directory

`saveScreenPNG()` at line 313-316 silently returns on `os.Create` error:
```go
f, err := os.Create(path)
if err != nil {
    return  // ← silent failure, no error logged
}
```

If `--screenshot /nonexistent/dir/out.png` is used, the file is never created
and no error is reported. The `--dump-keyframes` path avoids this because it
calls `os.MkdirAll` at line 168.

### 3b. Screenshot timing vs keyframes

`--screenshot` runs AFTER all frames (line 262-264). It captures the final
frame state, which includes any HALT. For che_cascade (which HALTs at the end),
this should be fine — the screen is frozen.

`--dump-keyframes` captures whenever VRAM changes (line 246-258), which catches
intermediate states. If the program writes to VRAM progressively, keyframes show
the progression while `--screenshot` only shows the end.

**The "flakiness" is likely:** `--screenshot` silently fails due to parent directory
not existing, while `--dump-keyframes` creates its directory automatically.

## 4. Smallest Concrete Fix

**Option A (no code change — correct invocation):**

```bash
# 1. Build headless
cd minzc && go build -tags mzx_headless -o mzx-headless ./cmd/mzx

# 2. Create white-on-black attributes
python3 -c "import sys; sys.stdout.buffer.write(b'\x07'*768)" > /tmp/attrs.bin

# 3. Compile che_cascade
./mz fun/che_cascade.nanz -o build/che_cas.a80
./mza build/che_cas.a80 -o build/che_cas.bin

# 4. Run with attrs preloaded, 500 frames (enough for one seed)
./mzx-headless --load /tmp/attrs.bin@0x5800 \
               --load build/che_cas.bin@0x8000 \
               --set "PC=8000,SP=FFFF,DI,IM=1" \
               --frames 500 \
               --screenshot build/che_cas_screenshot.png
```

**Option B (tiny code fix — 2 lines):**

Add to `main_headless.go` after the `--set` block (after line 153):

```go
// Initialize attributes to white-on-black if no snapshot loaded and screen is blank
if *snapshotFlag == "" {
    for i := 0x1800; i < 0x1B00; i++ {
        machine.Memory.Write(uint16(0x4000+i), 0x07, false) // ink=white, paper=black
    }
}
```

This gives a sane default for programs that write bitmap but not attributes.

**Option C (fix in che_cascade.nanz — 3 lines):**

Add at the start of `main()`:
```nanz
// Set attributes: white ink, black paper
for i in 0..768 {
    let p: ^u8 = 0x5800 + i
    p^ = 7
}
```

**Recommendation:** Option A for immediate testing, Option C for the program itself.
Option B is a reasonable default but changes emulator semantics.
