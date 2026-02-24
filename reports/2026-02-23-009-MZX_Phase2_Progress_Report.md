# MZX Phase 2 Progress Report

## Summary

Phase 2 of the MZX ZX Spectrum emulator adds media format support, sound chip emulation, and screenshot tooling to the Phase 1 foundation.

## Completed Features

### 1. AY-3-8912 Sound Chip (`ay.go`, ~630 lines)

Full Go port of **AYumi** by Peter Sovietov (MIT license). High-fidelity emulation:

- 3 tone channels + 1 noise channel + envelope generator
- 16 hardware registers ($FFFD register select, $BFFD data write)
- Cubic interpolation + 192-tap FIR decimation (AYumi's signature quality)
- DC removal filter for clean output
- Both AY-3-8910 and YM2149 DAC tables supported
- Default ACB stereo panning (A=left, B=center, C=right)

**Timing Integration**: AY samples are now **frame-synchronized** — `AY.EndFrame()` is called from `Machine.RunFrame()` alongside `Beeper.EndFrame()`. This generates exactly 882 samples per frame (44100Hz / 50Hz) into a frame buffer, which the audio mixer drains. Previously samples were generated on-demand from the audio callback thread, which was timing-decoupled.

### 2. Audio Mixer (`main.go`)

The `audioMixer` mixes beeper (mono, 60% volume) and AY (stereo, 80% volume) with clamping. Both sources are frame-synchronized: beeper samples come from `Beeper.EndFrame()`, AY samples from `AY.EndFrame()`. The mixer reads from their respective frame buffers in the Ebitengine audio callback.

### 3. .tap Tape Format (`formats/tap.go`, ~164 lines)

ROM trap at `$0556` (LD-BYTES routine in 48K ROM):

- Parses .tap files: 2-byte block length + flag + payload + checksum
- Intercepts tape loading via register convention:
  - A = expected flag byte ($00 header, $FF data)
  - F.carry = LOAD (set) or VERIFY (clear)
  - IX = destination address
  - DE = expected block length
- On match: copies data to RAM, sets carry flag, simulates RET
- Sequential block advancement with rewind support

### 4. .trd Disk Format (`formats/trd.go`, ~470 lines)

**Full TR-DOS function dispatch** via PC trap at `$3D13`. The C register selects the function:

| Fn   | Description             | Registers                              |
|------|-------------------------|----------------------------------------|
| $00  | Interface init          | —                                      |
| $01  | Drive init              | A=drive                                |
| $02  | Seek track              | A=track                                |
| $03  | Set sector              | A=sector                               |
| $04  | Set buffer address      | HL=addr                                |
| $05  | **Read sectors**        | HL=dest, D=track, E=sector, B=count   |
| $06  | **Write sectors**       | HL=src, D=track, E=sector, B=count    |
| $07  | Catalog display         | A=stream                               |
| $08  | Read file descriptor    | A=index -> $5CDD                       |
| $09  | Write file descriptor   | A=index <- $5CDD                       |
| $0A  | **Find file**           | $5CDD=name -> C=index/$FF             |
| $0B  | **Save CODE file**      | HL=start, DE=length                    |
| $0C  | Save BASIC              | —                                      |
| $0E  | **Load/verify file**    | A=mode, $5CDD=descriptor              |
| $12  | Delete file             | $5CDD=name                             |
| $13  | Copy to desc area       | HL=src -> $5CDD                        |
| $14  | Copy from desc area     | HL=dest <- $5CDD                       |
| $15  | Test track              | D=track                                |
| $16  | Select bottom side      | —                                      |
| $17  | Select top side         | —                                      |
| $18  | Read system sector      | —                                      |

This covers the most critical functions used by games:
- **Sector-level I/O** ($05/$06) — used by custom loaders that bypass the filesystem
- **File-level ops** ($0A/$08/$0E) — used by standard TR-DOS loading sequences
- **Save support** ($0B) — write CODE files back to disk image

### 5. Conditional Screenshots (`main.go`)

Screenshot modes for headless automation:

| Flag                      | Trigger                                    |
|---------------------------|--------------------------------------------|
| `--screenshot file.png`   | Basic: save after N frames (default)       |
| `--screenshot-on-halt`    | Save when CPU executes HALT instruction    |
| `--screenshot-on-stable N`| Save when screen unchanged for N frames    |
| `--screenshot-at-pc ADDR` | Save when PC reaches address (hex)         |
| `--max-frames N`          | Timeout for conditional triggers (default: 5000) |

Examples:
```bash
# Simple: capture after 100 frames
mzx --rom 48.rom --snapshot game.sna --screenshot shot.png --frames 100

# Wait for program to halt
mzx --rom 48.rom --snapshot game.sna --screenshot shot.png --screenshot-on-halt

# Wait until screen is stable (e.g., loading finished)
mzx --rom 48.rom --tap game.tap --screenshot shot.png --screenshot-on-stable 3

# Capture at specific program counter
mzx --rom 48.rom --snapshot game.sna --screenshot shot.png --screenshot-at-pc 8000
```

In-game screenshot: press **F2** to save `mzx_screenshot_NNNNNN.png`.

## Architecture Notes

### AY Timing Flow

```
Machine.RunFrame()
  ├── CPU.Interrupt()          // Frame start
  ├── loop: DoOpcode() + ULA.StepTo()
  ├── ULA.EndFrame()           // Flash toggle, finalize pixels
  ├── Beeper.EndFrame()        // Downsample beeper to 882 samples
  └── AY.EndFrame()            // Render 882 stereo samples via AYumi

Audio callback (Ebitengine thread):
  audioMixer.Read()
  ├── Beeper.ReadSamples()     // Drain beeper frame buffer
  ├── AY.ReadFrameSamples()    // Drain AY frame buffer
  └── Mix & convert to 16-bit stereo PCM
```

### TR-DOS Trap Architecture

```
Z80 executes CALL $3D13
  → PC trap fires
  → TRDState.dispatch() reads C register
  → Routes to function handler
  → Handler reads disk image / writes RAM
  → Sets BC (error code)
  → doTRDRet() pops return address from stack
  → Execution resumes after the CALL
```

## File Sizes

| File              | Lines | Description                    |
|-------------------|-------|--------------------------------|
| ay.go             | ~630  | AYumi port + frame buffer      |
| formats/tap.go    | ~164  | .tap ROM trap                  |
| formats/trd.go    | ~470  | .trd full TR-DOS dispatch      |
| machine.go (edit) | +3    | AY.EndFrame() call             |
| ports.go (edit)   | +30   | AY port registration           |
| cmd/mzx/main.go   | ~510  | All CLI features + mixer       |
| **Phase 2 total** | ~800  | New code added                 |

## Test Results

- All 26 spectrum package tests pass
- FUSE Z80 test suite: 1335/1335 (unchanged)
- Build: clean (only Ebitengine macOS deprecation warnings)
