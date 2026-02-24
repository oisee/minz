# MZD: IDA Pro-Like Z80 Disassembler & Analysis Engine

**Status:** Complete (v0.18.0)
**Commits:** `76d0dd3`, `81cb36b`

## Summary

MZD evolved from a linear disassembler into a **production-grade IDA Pro equivalent for Z80 binaries**. Two major features were added: recursive descent analysis with cross-references, and an extensible ABI annotation system with platform profiles.

## Feature 1: Recursive Descent Analysis Engine

**Location:** `minzc/pkg/disasm/analysis/`

### How It Works
1. **Entry point seeding** — platform-aware: CP/M starts at `$0100`, generic adds RST vectors + NMI (`$0066`)
2. **Queue-based recursive descent** — walks basic blocks, tracks instruction boundaries
3. **Instruction classification** — 10 types: normal, unconditional/conditional jumps, calls, RST, returns, halt, indirect jump
4. **Function boundary detection** — CALL/RST targets become function entries
5. **Cross-reference tracking** — bidirectional xrefs for control flow (call/jump) and data (read/write)

### ByteMap Classification
Every address gets one of 5 states:
- `ByteUndefined` — not yet analyzed
- `ByteCodeStart` — first byte of instruction
- `ByteCode` — interior byte of multi-byte instruction
- `ByteData` — known data
- `ByteString` — detected ASCII string

### String Detection
Scans undefined regions for printable ASCII (min 4 chars), recognizing:
- NUL terminator (`$00`) — C-style
- CR terminator (`$0D`) — CP/M
- `$` terminator (`$24`) — CP/M BDOS
- Bit-7 set (`$80`) — ZX Spectrum convention

### T-State Cycle Counting
Exact cycle counts for every Z80 instruction, including conditional variants (taken vs not-taken). All prefixes covered: CB, DD, FD, ED, DDCB, FDCB.

## Feature 2: ABI Annotation System

### The `.abi` File Format
INI-style text defining system call entry points and signatures:

```ini
[platform]
name=CP/M 2.2
arch=z80

[entry:CALL:0x0005]
name=BDOS
dispatch=C
2=CONSOLE_OUTPUT,Write char to console,E:char

[entry:RST:0x10]
name=PUTCHAR
desc=Output character
params=A:char
```

### Built-In Platform Profiles (4)
| Platform | Entry Points | Functions |
|----------|-------------|-----------|
| **CP/M 2.2** | BDOS `$0005` | 40+ (file I/O, console, disk) |
| **ZX Spectrum** | RST vectors + ROM calls | Graphics, keyboard, beeper, tape |
| **MSX** | VDP, PSG, input BIOS calls | ~30 system calls |
| **Agon Light 2** | MOS RST `$00` (29+ funcs), RST `$10`/`$18` | File I/O, UART, RTC |

### Backward Parameter Scanning
Walks backward from CALL/RST up to 8 instructions, looking for immediate loads to the dispatch register. Detects register clobbers to avoid false matches. Generates annotations like:
```
[ABI] BDOS #2: CONSOLE_OUTPUT — Write char to console (E=$48 'H')
```

## Feature 3: Project Persistence

### `.mzp` Format (JSON)
Saves complete analysis sessions:
- User labels, comments, code/data overrides
- Entry points, platform detection
- Auto-regenerates auto-labels on reload

### Symbol Management
Three-tier priority: User > Platform > Auto
- Functions: `sub_NNNN`
- Jump targets: `loc_NNNN`
- Strings: `str_NNNN`
- Data refs: `dat_NNNN`
- Vectors: `vec_RST00`, `vec_NMI`

## CLI Usage

```bash
mzd program.bin --analyze                    # Recursive descent analysis
mzd program.com --analyze -t cpm --cycles    # CP/M binary with T-states
mzd program.bin --analyze -R                 # Reassemblable output
mzd program.bin --abi custom.abi             # Custom ABI definitions
mzd program.bin --project analysis.mzp       # Save/load sessions
mzd program.bin --export-abi profile.abi     # Export platform profile
```

## Architecture

| File | Purpose |
|------|---------|
| `analysis.go` | Core types, recursive descent engine |
| `engine.go` | Instruction classification, control flow |
| `abi.go` | System call annotation, backward scanning (~900 lines) |
| `abi_format.go` | `.abi` file parsing/exporting |
| `symbols.go` | Label/symbol management |
| `labels.go` | Auto-labeling, priority system |
| `project.go` | `.mzp` project persistence |
| `data.go` | String & data block detection |
| `cycles.go` | T-state counting for all Z80 instructions |
| `xref.go` | Cross-reference tracking |
