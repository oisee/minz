# fun_backend/ — Backend Playground

Hands-on demos for MinZ's multi-target backend. One source → many targets.

## Quick Start

```bash
cd minzc   # all commands run from minzc/

# 1. See LLVM IR from Nanz
./minzc ../fun_backend/fibonacci.nanz --emit llvm

# 2. Native x86_64 binary (via clang)
./minzc ../fun_backend/fibonacci.nanz --emit llvm -o /tmp/fib.ll
echo 'define i32 @_m(){%r=call i32 @main() ret i32 %r}' >> /tmp/fib.ll
clang -x ir /tmp/fib.ll -o /tmp/fib
/tmp/fib; echo "fib(10) = $?"   # → 55

# 3. lli JIT (no compile step!)
lli -entry-function=_m /tmp/fib.ll; echo "= $?"

# 4. Native via QBE
mzn ../fun_backend/fibonacci.nanz --qbe

# 5. Native via C99
mzn ../fun_backend/fibonacci.nanz --c99

# 6. Show generated C code
mzn ../fun_backend/fibonacci.nanz --emit-c

# 7. Show generated QBE IL
mzn ../fun_backend/fibonacci.nanz --emit-qbe

# 8. MIR2 VM (no codegen, pure interpretation)
mzv ../fun_backend/fibonacci.nanz --headless

# 9. Z80 → emulator
./minzc ../fun_backend/fibonacci.nanz -o /tmp/fib.a80
mza /tmp/fib.a80 -o /tmp/fib.bin
mze /tmp/fib.bin

# 10. WebAssembly text
./minzc ../fun_backend/fibonacci.nanz --emit wasm
```

## Programs

### fibonacci.nanz
Recursive fibonacci. Compiles to all 8 backends. `fib(10) = 55`.

### rosetta.nanz
Correctness oracle: abs_diff, max, min, clamp, gcd. Run on all 4 assert backends:
```bash
./minzc ../fun_backend/rosetta.nanz --asserts-force mir2   # MIR2 VM
./minzc ../fun_backend/rosetta.nanz --asserts-force z80    # Z80 emulator
./minzc ../fun_backend/rosetta.nanz --asserts-force wasm   # WebAssembly
./minzc ../fun_backend/rosetta.nanz --asserts-force llvm   # LLVM JIT
```
If all 4 pass → algorithm is provably correct across backends.

### seven_targets.nanz
Demonstrates all 7 compilation targets from one source file.

### inspect_ir.nanz
Compare 5 intermediate representations:
```bash
./minzc ../fun_backend/inspect_ir.nanz --emit hir        # HIR (typed AST)
./minzc ../fun_backend/inspect_ir.nanz --emit mir2-raw   # MIR2 before opts
./minzc ../fun_backend/inspect_ir.nanz --emit mir2       # MIR2 after opts
./minzc ../fun_backend/inspect_ir.nanz --emit llvm       # LLVM IR
./minzc ../fun_backend/inspect_ir.nanz --emit wasm       # WebAssembly
```

### mul_benchmark.c
C program: multiply, square, cube, dot product. Compare Z80 T-states vs native.

### abap_on_x86.abap
ABAP fibonacci running natively on x86_64 via LLVM. Not SAP, not Z80 — real metal!

## Tools Reference

| Tool | What it does | Example |
|------|-------------|---------|
| `minzc` | Compiler (all frontends → Z80) | `minzc prog.nanz -o prog.a80` |
| `mzn` | Native compiler (→ x86_64 via QBE/C99) | `mzn prog.nanz --qbe` |
| `mze` | Z80 emulator (1335/1335 FUSE) | `mze prog.bin` |
| `mzv` | MIR2 VM with TUI display | `mzv prog.nanz` |
| `mzx` | ZX Spectrum emulator (GUI) | `mzx prog.sna` |
| `mza` | Z80 assembler | `mza prog.a80 -o prog.bin` |
| `mzd` | Z80 disassembler | `mzd prog.bin --cycles` |
| `lli` | LLVM JIT interpreter | `lli prog.ll` |
| `clang` | LLVM → native binary | `clang prog.ll -o prog` |
| `opt` | LLVM optimizer | `opt -O2 prog.ll -o opt.ll` |

## Emit Formats

| Format | Flag | Output |
|--------|------|--------|
| HIR | `--emit hir` | Typed AST (high-level) |
| MIR2 raw | `--emit mir2-raw` | SSA before optimization |
| MIR2 | `--emit mir2` | SSA after optimization |
| LLVM IR | `--emit llvm` | LLVM text (.ll) |
| WAT | `--emit wasm` | WebAssembly text |
| Nanz | `--emit nanz` | Reconstructed Nanz source |
| Lanz | `--emit lanz` | S-expression form |

## Assert Backends

Verify correctness on 4 independent execution engines:
```
--asserts-force mir2   # MIR2 VM (Go, fast)
--asserts-force z80    # Z80 emulator (cycle-accurate)
--asserts-force wasm   # WebAssembly (wazero, Go-native)
--asserts-force llvm   # LLVM (llvmlite JIT or lli)
```

## Backend Stats (2026-03-29)

- **42 native x86_64 binaries** from 8 frontends
- **ABAP**: 23/30 programs compile to native (77%)
- **Nanz**: 9/10 self-contained pass lli (90%)
- **Pascal**: 7/9 pass (78%)
- **8 backends**: Z80, eZ80, LLVM, WASM, QBE, C99, CUDA, VIR/Z3
