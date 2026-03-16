# FatFS — C Readiness Benchmark

FatFS R0.16 by ChaN (elm-chan.org/fsw/ff/)
License: open source, see ff.c header

## Purpose

Real-world C code to benchmark MinZ C89 frontend readiness.
FatFS is the standard FAT filesystem driver for embedded systems.
On SDCC for Z80, it compiles to 16KB+ binary.

## Source files (from ff16.zip)

- ff.c — main FAT driver (~7K lines)
- ff.h — public API
- ffconf.h — configuration
- diskio.h — disk I/O interface
- diskio.c — disk I/O stubs
- ffsystem.c — OS-dependent functions

## Key C constructs used

| Construct | Count | MinZ Status |
|-----------|-------|-------------|
| do-while loops | 49 | BUG: infinite loop in constprop |
| `++`/`--` ops | 268 | needs verification |
| address-of `&var` | 169 | partial (globals only) |
| ternary `?:` | 83 | OK |
| memcpy/memset/memcmp | 75 | need string.h stub |
| preprocessor `#if` | 260 | OK (cparse) |
| switch statements | 5 | OK |
| struct typedefs | ~15 | OK |
| multi-dim arrays | 0 | N/A |
| goto | 0 | N/A |

## Readiness tests

See `fatfs_constructs.c` for targeted tests of constructs FatFS needs.
