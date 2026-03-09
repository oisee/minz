# plm80c — Intel PL/M-80 V4.0 Compiler (C port)

Build script and test files for **plm80c**, Mark Ogden's C port of the original
Intel PL/M-80 V4.0 compiler from `ogdenpm/c-ports`.

Used to generate reference 8080 output for comparison against our MIR2 Z80 backend.
See `reports/2026-03-09-036-Native_PLM80_vs_MIR2_Codegen_Comparison.md`.

## Build

```bash
./build.sh           # clones, patches, builds → /tmp/plm80c
```

Requires: `git`, `gcc`. Tested on macOS arm64 and Linux x86_64.

### macOS patches applied automatically

Three source patches are required on macOS (applied in-place by `build.sh`):

1. **`utility/option.c`** — prepend `#include <limits.h>` (CHAR_BIT undefined on macOS)
2. **`shared/os.c`** — add `fcloseall()` stub (`#ifdef __APPLE__` guard)
3. **Link** — omit `exit.o` (duplicate `_Exit` symbol conflict with `os.o`)

## Compile a PL/M-80 file

```bash
# Compile with annotated listing (shows opcodes alongside source)
/tmp/plm80c CODE SYMBOLS report_demo.plm

# This produces report_demo.obj (Intel OMF)
# To get a .com binary, link with link80:
#   link80 report_demo.obj TO report_demo.com
```

The `CODE SYMBOLS` flags produce an annotated `.lst` listing file — the most
useful output for comparing against our assembler output.

## Notes

- PL/M-80 V4.0 targets the **Intel 8080**, not Z80. No `CP reg`, `NEG`, `DJNZ`,
  `EX DE,HL` etc. The 8080 subset is a strict subset of Z80 binary-compatible.
- **Calling convention**: parameters stored to absolute memory addresses at
  procedure entry via `LXI H,addr` + `MOV M,reg`. Every variable access is a
  memory round-trip.
- **PL/M-80 module syntax**: source must be wrapped in `NAME: DO; ... END NAME;`
- **Declarations**: all `DECLARE` statements must precede executable statements
  within a block (unlike MinZ/Nanz where `let` can appear anywhere)

## Files

| File | Description |
|------|-------------|
| `build.sh` | Build script with macOS patches |
| `report_demo.plm` | Test file: `abs_diff` + `fib` (PL/M-80 conformant) |
