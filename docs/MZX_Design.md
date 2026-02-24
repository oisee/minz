# MZX Design — ZX Spectrum Emulator

MZX is the MinZ toolchain's ZX Spectrum emulator. It's a T-state accurate emulator built on Ebitengine (graphics/input) and oto (audio), wrapping a remogatto/z80 CPU core with custom ULA, memory, and I/O subsystems.

**Source:** `minzc/cmd/mzx/main.go` + `minzc/pkg/spectrum/`

---

## Architecture

```
┌──────────────────────────────────────────────┐
│                    MZX                        │
│                                               │
│  ┌──────────┐  ┌──────┐  ┌────────────────┐  │
│  │ Ebitengine│  │ oto  │  │  CLI / Headless│  │
│  │ (window)  │  │(audio)│  │  (screenshots) │  │
│  └─────┬─────┘  └──┬───┘  └───────┬────────┘  │
│        │           │               │           │
│  ┌─────▼───────────▼───────────────▼────────┐  │
│  │           spectrum.Machine               │  │
│  │                                          │  │
│  │  CPU ──── Memory ──── ULA ──── Ports     │  │
│  │   │                    │         │       │  │
│  │   │       Keyboard ────┘    ┌────┘       │  │
│  │   │                        │             │  │
│  │   └── Beeper ─────── AYChip             │  │
│  └──────────────────────────────────────────┘  │
│                                               │
│  ┌──────────────────────────────────────────┐  │
│  │            formats package               │  │
│  │  .tap  .trd/.scl  .sna  console  BASIC  │  │
│  └──────────────────────────────────────────┘  │
└──────────────────────────────────────────────┘
```

### Components

| Component | File | Description |
|-----------|------|-------------|
| **Machine** | `machine.go` | Orchestrates frame execution, connects all subsystems |
| **CPU** | `cpu.go` | `CPUCore` interface wrapping remogatto/z80 (all Z80 ops including undocumented) |
| **Memory** | `memory.go` | 64KB address space, 128K paging, contention timing |
| **ULA** | `ula.go`, `ula_timing.go` | T-state accurate video: border, screen, attribute flash, contention |
| **Ports** | `ports.go` | I/O port dispatch ($FE keyboard/border/beeper, $FFFD/$BFFD AY, $7FFD paging) |
| **Keyboard** | (in ports) | 8-row matrix, directly mapped from Ebitengine key state |
| **Beeper** | `beeper.go` | 1-bit audio from port $FE bit 4, sample-accurate edge detection |
| **AY** | `ay.go` | AY-3-8912 PSG: 3 tone channels + noise + envelopes, stereo panning (ACB) |
| **Formats** | `formats/` | .tap (tape), .trd/.scl (TR-DOS disk), .sna (snapshot), BASIC tokenizer, console capture |

---

## Machine Models

| Model | Flag | Timing | Memory | Features |
|-------|------|--------|--------|----------|
| **48K** | `--model 48k` | 69888 T/frame, 48 border top | 48KB RAM + 16KB ROM | Standard Spectrum |
| **128K** | `--model 128k` | 70908 T/frame, 63 border top | 128KB RAM + 32KB ROM | AY, paging, 2 ROMs |
| **Pentagon** | `--model pentagon` | 71680 T/frame, 64 border top | 128KB RAM + 32KB ROM | Russian clone timing |

---

## Audio System

Two independent audio sources mixed to stereo PCM via oto (direct CoreAudio/ALSA):

### Beeper
- 1-bit output from port $FE bit 4
- Sample-accurate: timestamps each edge within the frame
- Generates waveform at 44100 Hz matching the exact T-state positions
- Disabled with `--no-beeper`

### AY-3-8912
- 3 tone channels (A/B/C) + 1 noise channel
- Hardware envelope generator (16 shapes)
- ACB stereo panning (A=left, C=center, B=right)
- Register access via ports $FFFD (select) / $BFFD (write)
- Disabled with `--no-ay`

### Mixing
- Beeper × 0.6 + AY × 0.8 per channel
- 40ms buffer (2 frames) for low latency without underruns
- Partial reads allowed (never blocks the audio thread)

---

## Video / ULA

T-state accurate rendering — the ULA draws pixels as the CPU executes:

- **Screen area**: 256×192 pixels at $4000-$57FF (bitmap) + $5800-$5AFF (attributes)
- **Border**: Colored ring around screen, set by port $FE bits 0-2
- **Contention**: Memory access at $4000-$7FFF adds wait states matching real hardware
- **Flash**: Attribute bit 7 toggles foreground/background every 16 frames

### Framebuffer sizes
- 48K: 352×296 (48px border top/bottom, 48px border left/right)
- Pentagon: 352×312 (64px border top, 56px border bottom)

---

## Keyboard Mapping

US keyboard layout → ZX Spectrum 8×5 matrix:

| PC Key | Spectrum | Notes |
|--------|----------|-------|
| A-Z | A-Z | Direct mapping |
| 0-9 | 0-9 | Direct mapping |
| Enter | ENTER | |
| Space | SPACE | |
| Backspace | CS+0 | DELETE |
| Arrows | CS+5/6/7/8 | Cursor keys |
| Escape | CS+1 | EDIT mode (break) |
| Tab | CS+SS | Extended mode |
| Left Shift | Caps Shift | Unless consumed by shifted punctuation |
| Right Shift | Symbol Shift | |
| Ctrl | Symbol Shift | Always |
| Shift+1-0 | SS+key | Shifted punctuation (!, @, #, etc.) |

Shifted punctuation (e.g. Shift+8 = `*`) automatically maps to the correct Spectrum Symbol Shift combo (SS+B for `*`), consuming the Shift so it doesn't also trigger Caps Shift.

### Function Keys
| Key | Action |
|-----|--------|
| F1 | Pause/unpause |
| F2 | Save screenshot |
| F3 | Toggle turbo (20× speed) |
| F4 | Hold for turbo |
| F5 | Reset machine |
| F12 | Save .sna snapshot |
| Ctrl+F14 / Pause-Break | *Future: Monitor/debugger mode* |

---

## CLI Flags

### Machine Setup
| Flag | Description |
|------|-------------|
| `--model` | Machine model: `48k`, `128k`, `pentagon` |
| `--rom` | Custom ROM file |
| `--rom1` | Second ROM (128K models) |
| `--scale` | Display scale: 1-4 (default 2) |

### Loading
| Flag | Description |
|------|-------------|
| `--snapshot` | Load .sna snapshot |
| `--tap` | Load .tap tape (instant trap loading) |
| `--tap-realtime` | Load tape in real-time with audio |
| `--trd` | Load .trd disk image |
| `--scl` | Load .scl disk image (auto-converts to TRD) |
| `--trd-load` | Load specific file from disk: `name:ext:addr` |
| `--trd-dir` | List disk directory and exit |
| `--load` | Load raw binary to memory: `FILE@ADDR` or `FILE@ADDR:PAGE` (repeatable) |
| `--run` | Load and run: `FILE@ADDR` (shortcut for `--load FILE@ADDR --set PC=ADDR,SP=FFFF,DI,IM=1`) |
| `--save-snapshot` | Save .sna snapshot after running frames (headless) |

### CPU Control
| Flag | Description |
|------|-------------|
| `--set` | Set CPU registers: `PC=8000,SP=FFFF,DI,IM=1` (hex values, comma-separated) |

Supported `--set` assignments:

| Name | Width | Description |
|------|-------|-------------|
| `PC`, `SP` | 16-bit | Program counter, stack pointer |
| `AF`, `BC`, `DE`, `HL` | 16-bit | Main register pairs |
| `IX`, `IY` | 16-bit | Index registers |
| `AF'`, `BC'`, `DE'`, `HL'` | 16-bit | Shadow register pairs |
| `A` | 8-bit | Accumulator (preserves flags) |
| `I`, `R` | 8-bit | Interrupt vector, refresh counter |
| `IM` | 8-bit | Interrupt mode (0, 1, or 2) |
| `DI` | command | Disable interrupts (IFF1=IFF2=false) |
| `EI` | command | Enable interrupts (IFF1=IFF2=true) |

### Automation
| Flag | Description |
|------|-------------|
| `--exec` | Execute BASIC command (tokenized): `'LOAD ""'` |
| `--type` | Inject keystrokes: `'LOAD ""\n'` |
| `--console` | Mirror BASIC RST $10 output to stdout |

### Audio
| Flag | Description |
|------|-------------|
| `--no-audio` | Disable all audio |
| `--no-beeper` | Disable beeper only |
| `--no-ay` | Disable AY chip only |

### Headless / Screenshot
| Flag | Description |
|------|-------------|
| `--screenshot` | Save single PNG and exit |
| `--frames` | Frame spec: count, range, trigger (see below) |
| `--dump-frames` | Save every frame to directory |
| `--dump-keyframes` | Save only changed frames |
| `--skip` | Turbo-skip to first capture frame |
| `--no-border` | 256×192 crop |
| `--full-border` | Full ULA output |
| `--max-frames` | Safety limit for headless (default 5000) |

### Frame Spec Syntax (`--frames`)
```
50                  # Run 50 frames
100..200            # Capture frames 100-200
100-200,500,800-900 # Multi-range
PC=4000             # Trigger when PC reaches $4000
PC=4000+50          # 50 frames after PC trigger
T=100000            # Trigger at T-state count
DI:HALT             # Trigger on dead CPU (DI + HALT)
```

---

## Initialization Order

```
1. Create machine (model selection, ROM loading)
2. Audio setup (disable beeper/AY per flags)
3. Load snapshot (--snapshot)
4. Apply --load files (raw binary → memory)       ← NEW
5. Apply --set registers (CPU state)               ← NEW
6. Install tape/disk traps (--tap, --trd, --scl)
7. Auto-load from tape (if no snapshot)
8. Console capture (--console)
9. BASIC execution (--exec)
10. Keystroke injection (--type)
11. Headless capture OR interactive window
```

Steps 4-5 happen after snapshot but before tape/disk, so `--load`+`--set` can fully control execution without ROM involvement.

---

## Typical Usage

```bash
# Interactive Spectrum
mzx

# Load and run a tape
mzx --tap game.tap

# Load compiled MinZ binary directly (long form)
mzx --load program.bin@8000 --set PC=8000,SP=FFFF,DI

# Load and run (shortcut — same thing)
mzx --run program.bin@8000

# Headless screenshot of a tape program
mzx --tap demo.tap --screenshot demo.png --frames 500

# Capture specific frames from a demo
mzx --tap demo.tap --dump-keyframes frames/ --frames 100..500 --skip

# 128K with disk image
mzx --model pentagon --trd game.trd --trd-load "GAME:C:32768"

# Load binary into specific 128K RAM page
mzx --model pentagon --load data.bin@C000:3 --set PC=8000
```

---

## File Structure

```
minzc/cmd/mzx/
├── main.go          # CLI, Game loop, flag parsing, headless capture
├── suppress_*.go    # stderr suppression (macOS CAMetalLayer warnings)
└── roms/
    └── 48.rom       # Embedded 48K ROM

minzc/pkg/spectrum/
├── machine.go       # Machine orchestrator, frame execution
├── cpu.go           # CPUCore interface + RemogattoAdapter
├── memory.go        # 64KB + 128K paging + contention
├── ula.go           # Video rendering (T-state accurate)
├── ula_timing.go    # Contention tables per model
├── ports.go         # I/O port dispatch
├── beeper.go        # 1-bit audio
├── ay.go            # AY-3-8912 sound chip
├── tape_signal.go   # Real-time tape audio provider
└── formats/
    ├── tap.go       # .tap tape loading + traps
    ├── trd.go       # .trd/.scl disk + TR-DOS traps
    ├── sna.go       # .sna snapshot format
    ├── console.go   # BASIC output capture
    └── basic.go     # BASIC tokenizer + exec
```
