# mir2llvm — MIR2 → LLVM IR Backend

Translates MIR2 SSA modules to LLVM IR text. Produces `.ll` files that can be:
- JIT-executed via `lli` (LLVM interpreter)
- Compiled to native binaries via `clang program.ll -o program`
- Optimized via `opt -O2 program.ll`
- Cross-compiled to any LLVM target (ARM64, RISC-V, WASM, etc.)

## Status

**42 native x86_64 binaries** from 8 frontends (ABAP, Nanz, Pascal, Lizp, Frill, C89, C, PL/M).

| Frontend | Native Binaries | Pass Rate |
|----------|:---:|:---:|
| ABAP | 23 | 92% |
| Nanz | 9 | 90% |
| Pascal | 7 | 78% |
| Lizp | 3 | 43% |

## Usage

```bash
# Emit LLVM IR
mz program.nanz --emit llvm -o program.ll

# JIT execute
lli program.ll

# Compile to native
clang program.ll -o program
./program

# Optimize then compile
opt -O2 program.ll -o program_opt.ll
clang program_opt.ll -o program

# Force asserts through LLVM
mz program.nanz --asserts-force llvm
```

## Design

- **Uniform i32**: All integer types (u8/u16/i8/i16/bool) → `i32`. Avoids LLVM type mismatches.
- **Named registers**: `%vN` format, no sequential numbering constraint.
- **Reaching definitions**: Forward dataflow for multi-block phi registers.
- **CMP zext**: `icmp` → `i1` → `zext i32`. Trunc back at `br i1`.
- **Ptr handling**: `ptrtoint`/`inttoptr` at call/ret boundaries.

## Files

| File | LOC | Purpose |
|------|:---:|---------|
| `codegen.go` | ~500 | MIR2 → LLVM IR text |
| `codegen_test.go` | ~170 | 11 tests (structure + lli native) |
| `runner.go` | ~160 | Assert execution via llvmlite (Python) |

## Supported MIR2 Ops

Arithmetic: Add, Sub, Mul, Div, SDiv, Mod
Bitwise: And, Or, Xor, Shl, Shr, Sar
Unary: Neg, Not
Compare: Cmp (all predicates)
Memory: Load, Store, Alloca, Field, PtrAdd, PtrBump, AddrOf
Convert: Ext, Sext, Trunc
Control: Call, Ret, Jmp, BrIf, CondRet, BrIf2
Data: Const, Move

## Known Limitations

- **OpAsm** (inline Z80 assembly): emits `; TODO` — Z80-specific, can't translate
- **External runtime**: programs using `_mir_io_print_*`, TUI, SQLite need linked stubs
- **Frill**: duplicate function names from pipe/compose desugaring (frontend bug)
