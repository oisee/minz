# MZX: T-State Accurate ZX Spectrum Emulator

**Status:** Phase 1 + Phase 2 Complete (v0.18.0)

## Summary

MZX is a T-state accurate ZX Spectrum emulator — a new component of the MinZ toolchain. Built on the proven remogatto/z80 CPU core (1335/1335 FUSE tests), it provides ULA display rendering, banked memory with contention, keyboard input, beeper + AY audio, .sna/.tap/.trd format support, and an Ebitengine graphical frontend at 50 Hz. Headless screenshot mode supports conditional triggers for automated testing and book illustration generation.

## Architecture

### Core Design Decisions
1. **CPUCore interface** — wraps remogatto/z80 now, swappable to eZ80 (Agon) or Z80Next later
2. **FrameMap-based ULA** — pre-computed per-T-state lookup table for exact screen/border timing
3. **Contention inside MemoryAccessor** — remogatto/z80 calls `ContendRead()` during `DoOpcode()` automatically
4. **VideoMode as data** — machine models are configuration structs (timing tables + dimensions)
5. **Frame-synchronized audio** — beeper and AY both render per-frame, drained by audio callback
6. **PC traps** — ROM/TR-DOS function interception without needing full ROM emulation

### Files (~3500 lines total)

| File | Lines | Description |
|------|-------|-------------|
| `cpu.go` | 140 | CPUCore interface + RemogattoAdapter |
| `ula_timing.go` | 247 | VideoMode structs, Mode48K/ModePentagon128, FrameMap generator |
| `memory.go` | 314 | Banked ROM/RAM, 48K flat + 128K paging, ULA contention |
| `keyboard.go` | 116 | 8x5 matrix keyboard (active low) |
| `beeper.go` | 126 | 1-bit audio, per-T-state sampling, 44.1 kHz output |
| `ula.go` | 259 | FrameMap ULA renderer, 16-color palette, flash support |
| `ports.go` | 252 | Mask-based dispatcher: ULA, paging, Kempston, AY |
| `machine.go` | 219 | Orchestrator with PC traps and AY integration |
| `ay.go` | 630 | AYumi Go port — AY-3-8912/YM2149 emulation |
| `formats/sna.go` | 125 | .sna snapshot loader |
| `formats/tap.go` | 164 | .tap tape with ROM trap at $0556 |
| `formats/trd.go` | 470 | .trd disk with full TR-DOS dispatch at $3D13 |
| `cmd/mzx/main.go` | 510 | Frontend with audio mixer, screenshots |
| `spectrum_test.go` | 502 | 26 unit tests |

### Machine Models

| Model | Clock | Lines/Frame | T-states/Frame | Contention | AY |
|-------|-------|-------------|----------------|------------|-----|
| 48K | 3.5 MHz | 312 | 69,888 | Yes (ULA) | No |
| Pentagon 128K | 3.55 MHz | 320 | 71,680 | No | Yes |

### Frame Execution Pipeline

```
Machine.RunFrame()
  ├── CPU.Interrupt()               // Maskable INT at frame start
  ├── loop:
  │   ├── Check PC traps            // ROM/TR-DOS interception
  │   ├── CPU.DoOpcode()            // Execute one instruction
  │   └── ULA.StepTo(tstates)       // Advance display rendering
  ├── ULA.EndFrame()                // Flash toggle every 16 frames
  ├── Beeper.EndFrame()             // Downsample to 882 samples (44.1kHz/50Hz)
  └── AY.EndFrame()                 // Render 882 stereo samples via AYumi

Audio callback (async, Ebitengine thread):
  audioMixer.Read()
  ├── Beeper.ReadSamples()          // Drain mono beeper frame buffer
  ├── AY.ReadFrameSamples()         // Drain stereo AY frame buffer
  └── Mix (60% beeper + 80% AY) → 16-bit stereo PCM
```

### TR-DOS Function Dispatch

Full implementation of the `CALL $3D13` API — C register selects function:

- **Sector I/O**: $05 (read), $06 (write) — used by custom game loaders
- **File ops**: $0A (find), $08/$09 (read/write descriptor), $0E (load), $0B (save CODE)
- **System**: $00/$01 (init), $02 (seek), $07 (catalog), $12 (delete)
- **Utility**: $13/$14 (copy descriptor area), $15 (test track), $16/$17 (side select)

## Usage

```bash
# Interactive emulation
./mzx --rom 48.rom --snapshot game.sna
./mzx --rom 48.rom --tap game.tap
./mzx --model pentagon --rom 128-0.rom --rom1 trdos.rom --trd game.trd
./mzx --rom 48.rom --trd game.trd --trd-load "GAME:C:32768"

# Headless screenshots
./mzx --rom 48.rom --snapshot game.sna --screenshot shot.png --frames 100
./mzx --rom 48.rom --snapshot game.sna --screenshot shot.png --screenshot-on-halt
./mzx --rom 48.rom --tap game.tap --screenshot shot.png --screenshot-on-stable 3
./mzx --rom 48.rom --snapshot game.sna --screenshot shot.png --screenshot-at-pc 8000
```

**Keys:** ESC=quit, F1=pause, F2=screenshot, F5=reset

## Phase 3 Roadmap

| Feature | Approach | Priority |
|---------|----------|----------|
| **DeZog/DZRP** | Debug protocol integration with mzd | High |
| **.z80 format** | Extended snapshot format (128K support) | Medium |
| **Scorpion 256K** | Additional machine model | Medium |
| **eZ80 core** | Agon Light 2 CPU swap via CPUCore | Future |
| **Z80Next** | ZX Next extended instructions | Future |

## Test Results

26/26 unit tests pass. FUSE: 1335/1335.
