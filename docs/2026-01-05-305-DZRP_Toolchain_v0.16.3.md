# DZRP Toolchain Release - v0.16.3

**Date:** January 5, 2026
**Codename:** "Instant Load Revolution"

## Executive Summary

This release introduces a complete DZRP (DeZog Remote Protocol) toolchain that transforms the ZX Spectrum development experience. Load 19KB programs instantly, debug remotely, and configure once for all tools.

## New Tools

### mztap - Instant TAP Loader

Load ZX Spectrum TAP files directly to any DZRP-compatible emulator, bypassing tape emulation entirely.

```bash
# List TAP contents
mztap --list game.tap

# Instant load and run (19KB in ~100ms!)
mztap game.tap

# Override addresses
mztap --load $9000 --start $9010 game.tap

# Set registers before execution
mztap --set "AF=0xFF,BC=$4000" game.tap
```

**Features:**
- TAP file parsing (PROGRAM, CODE, NUMARRAY, CHARARRAY blocks)
- CODE block extraction with load addresses
- Address override with flexible formats (0x, $, decimal, octal)
- Register initialization before execution
- List mode for TAP inspection

### mzrun Enhancements

The MinZ remote runner now supports flexible address formats and unified configuration.

```bash
# Hex formats
mzrun program.bin --load 0x8000
mzrun program.bin --load $9000

# Decimal and octal
mzrun program.bin --load 32768
mzrun program.bin --load 0100000  # Octal
```

## Unified Environment Variables

Configure once, use everywhere:

```bash
# Add to ~/.bashrc or ~/.zshrc
export DZRP_HOST=192.168.1.100   # Emulator host
export DZRP_PORT=11000           # DZRP port
export DZRP_SOCKET=tcp           # tcp or ws (WebSocket)

# All tools respect these settings
mzrun game.minz      # Uses DZRP_HOST:DZRP_PORT
mztap game.tap       # Uses DZRP_HOST:DZRP_PORT
```

## Address Format Support

Both tools support multiple address formats common in Z80 development:

| Format | Example | Value |
|--------|---------|-------|
| Hex (0x) | `0x8000` | 32768 |
| Hex ($) | `$8000` | 32768 |
| Decimal | `32768` | 32768 |
| Octal | `0100000` | 32768 |

## WebSocket Preparation

The `--socket` flag and `DZRP_SOCKET` environment variable prepare for future WebSocket support:

```bash
# Future: Connect through WebSocket
mzrun --socket ws program.minz
```

This will enable:
- Browser-based development tools
- Cloud IDE integration (Gitpod, Codespaces)
- Firewall/proxy traversal

## Tested Emulators

| Emulator | Status | Notes |
|----------|--------|-------|
| ZXSpeculator | ✅ Excellent | Native DZRP, recommended |
| ZEsarUX | ✅ Works | Built-in DZRP support |
| CSpect | ✅ Works | Requires DeZog plugin |

## Performance Comparison

Traditional tape loading vs mztap instant load:

| File Size | Tape Emulation | mztap |
|-----------|----------------|-------|
| 5KB | ~15 seconds | <50ms |
| 19KB | ~55 seconds | ~100ms |
| 48KB | ~2.5 minutes | ~200ms |

**Result:** 500-1000x faster loading!

## ZXSpeculator Integration

We've prepared recommendations for ZXSpeculator to adopt the same environment variables:

- `docs/304_ZXSpeculator_DZRP_Recommendations.md`

This will create a unified ecosystem where all DZRP tools share configuration.

## Future Vision: Shader Library

Inspired by ZXSpeculator's experiments (OneSmallStep, FireFX, etc.), we're exploring:

- **minz-glsl**: GLSL-style vector math library for MinZ
- Fixed-point vec2, vec3, vec4 types optimized for Z80
- Raymarching and SDF primitives
- Dithering algorithms (Floyd-Steinberg, ordered)

This could enable shader-like effects on vintage hardware!

## Installation

```bash
cd minz/minzc
make all  # Builds mz, mza, mze, mzr, mzrun, mztap
make install-user  # Install to ~/bin
```

## Breaking Changes

None - fully backward compatible.

## Contributors

- MinZ Team
- ZXSpeculator integration inspired by DeanTheCoder's excellent work

---

**Full Changelog:** v0.16.2...v0.16.3
