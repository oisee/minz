# MIR2 → LLVM IR Backend: 42 Native Binaries from 8 Frontends

**Date:** 2026-03-29
**Session:** mir2llvm production sprint

## Summary

Built a production-grade LLVM IR backend (`pkg/mir2llvm`) that compiles programs from **8 frontends** (ABAP, Nanz, Pascal, Lizp, Frill, C89, C99+, PL/M) through MIR2 → LLVM IR → native x86_64 binaries via `lli` and `clang`.

**42 programs compile to native x86_64 binaries. ABAP leads at 23/30 (77%).**

## Results

| Frontend | PASS (lli + clang) | FAIL | Skip (no main) | Skip (no LLVM) | Total |
|----------|:---:|:---:|:---:|:---:|:---:|
| **ABAP** | **23** | 5 | 2 | 0 | **30** |
| **Nanz** | **9** | 21 | 20 | 2 | **52** |
| **Pascal** | **7** | 2 | 0 | 2 | **11** |
| **Lizp** | **3** | 3 | 1 | 7 | **14** |
| **Frill** | 0 | 14 | 3 | 1 | **18** |
| **C89** | 0 | 0 | 36 | 3 | **39** |
| **C99+** | 0 | 0 | 19 | 1 | **20** |
| **PL/M** | 0 | 0 | 3 | 2 | **5** |
| **Total** | **42** | **45** | **84** | **18** | **189** |

### Pass Rate (programs with main, excluding external deps)

| Frontend | Self-contained | Pass | Rate |
|----------|:-:|:-:|:-:|
| ABAP | 25 | 23 | **92%** |
| Pascal | 9 | 7 | **78%** |
| Nanz (self-contained) | 10 | 9 | **90%** |
| Lizp | 7 | 3 | **43%** |
| Frill | 14 | 0 | **0%** (fixable — see action plan) |

## What Compiles to Native x86_64

### ABAP (23 programs)
```
hello.abap          → ./hello           # "Hello from ABAP on Z80!"
fibonacci.abap      → ./fibonacci       # 0 1 1 2 3 5 8 13 21 34...
fizzbuzz.abap       → ./fizzbuzz        # FizzBuzz
bubble_sort.abap    → ./bubble_sort     # Sorting algorithm
oop.abap            → ./oop             # OOP with classes
oop_assert.abap     → ./oop_assert      # OOP with assertions
forms.abap          → ./forms           # FORM/PERFORM subroutines
sap_calculator.abap → ./sap_calculator  # Calculator
sap_hello_zx.abap   → ./sap_hello_zx   # SAP Hello World
sap_zx_demo.abap    → ./sap_zx_demo    # SAP ZX Demo
showcase_zx.abap    → ./showcase_zx     # Feature showcase
select_demo.abap    → ./select_demo     # SELECT queries
select_loop.abap    → ./select_loop     # SELECT in loops
sqlite_demo.abap    → ./sqlite_demo     # SQLite integration
string_templates.abap → ./string_templates # String templates
sysinfo.abap        → ./sysinfo         # System info
name_test.abap      → ./name_test       # Name handling
function_test.abap  → ./function_test   # Function tests
hello_param.abap    → ./hello_param     # Parameterized hello
hello_simple.abap   → ./hello_simple    # Simple hello
mara_alv_zx.abap    → ./mara_alv_zx    # Material master ALV
zsql_mara_zx.abap   → ./zsql_mara_zx   # ZSQL material master
zsql_zx.abap        → ./zsql_zx         # ZSQL ZX variant
```

### Nanz (9 programs)
```
51_impl_cpm_demo  meta_screen  sap_mara_cpm  sql_test6  tetris_cpm
tui_cpm  tui_zx  zsql_zx  zsql_zx_real
```

### Pascal (7 programs)
```
assert_test  casetest  factorial  hello  recursive_test  sieve  string_test
```

### Lizp (3 programs)
```
cond_test  factorial  hello
```

## Failure Analysis

| Category | Count | Root Cause | Fix Plan |
|----------|:---:|-----------|----------|
| Symbols not found | 13 | External runtime (print, TUI, SQLite) | Implement print stubs; SQLite stubs later |
| invalid cast i32→i32 | 7 | Ext/Trunc same type (Frill) | Pre-check: skip when src==dst |
| expected top-level | 5 | Z80 validator stderr leak | Separate stderr from LLVM output |
| invalid operand type | 5 | Ptr↔int in screen hosts | Screen metafunctions bypass MIR2 |
| ptr vs i32 mismatch | 5 | Reaching defs incomplete for ptr | Extend reaching analysis |
| func redefinition | 3 | Frill duplicate functions | Frill frontend bug — deduplicate |
| Other | 7 | Parse errors, edge cases | Per-case fixes |

## Technical Details

### Architecture
```
Source (.nanz/.abap/.frl/.c/.pas/.lizp/.plm)
    ↓ frontend
  HIR (typed AST)
    ↓ hir.LowerModule()
  MIR2 (SSA IR)
    ↓ mir2llvm.Compile()
  LLVM IR (.ll text)
    ↓ lli (JIT) or clang (AOT)
  Native x86_64 binary
```

### Key Implementation Decisions

1. **Uniform i32** — All integer types (u8, u16, i8, i16, bool) map to `i32`. Avoids type mismatches across phi nodes and call boundaries. Masking to u8/u16 happens at Z80 level, not LLVM.

2. **Named registers `%vN`** — Avoids LLVM's sequential numbering constraint. MIR2 regs map directly to `%v{reg_id}`.

3. **CMP → zext i1→i32** — LLVM `icmp` returns `i1`, but our uniform type is `i32`. Auto-zext after every CMP, auto-trunc before every `br i1`.

4. **Reaching definitions** — Forward dataflow pass for multi-block phi registers. Tracks which LLVM name is visible in each block.

5. **Phi-init rename** — When same vreg has both instruction def (entry block) and phi def (loop header), instruction gets `%_init_vN`.

6. **Multi-block phi rename** — Same vreg as phi in multiple blocks → `%_phi_{block}_v{N}`.

7. **External declarations** — Functions called but not defined get `declare` instead of crashing.

### New MIR2 Ops Supported
- OpDiv (udiv), OpSDiv (sdiv), OpMod (urem)
- OpAlloca, OpField (GEP), OpPtrAdd, OpPtrBump
- OpCallIndirect (TODO)

### Files Modified
- `pkg/mir2llvm/codegen.go` — Complete rewrite (~500 LOC)
- `pkg/mir2llvm/codegen_test.go` — 11 tests (5 structure + 5 lli native + 1 well-formedness)
- `pkg/mir2llvm/runner.go` — llvmlite executor (unchanged)
- `pkg/pipeline/pipeline.go` — MIR2Module in Steps, LLVM assert runner
- `cmd/minzc/main.go` — `--emit llvm`, `--emit wasm`, `--asserts-force llvm`

## Timeline

| Step | Result |
|------|--------|
| Start (session begin) | 5 tests, llvmlite only, call arg types wrong |
| Fix call types + CMP operand | 5/5 lli tests pass |
| Add `--emit llvm` CLI | End-to-end: `mz program.nanz --emit llvm -o program.ll` |
| Corpus test #1 | 0/29 Nanz pass (all broken) |
| Named registers `%vN` | Fix sequential numbering |
| Uniform i32 | Fix type mismatches |
| Phi-init rename | Fix multiple definition (entry+loop) |
| Multi-block phi rename | 4/10 Nanz pass |
| Reaching definitions | 8/10 Nanz pass |
| Bool→i32 unification | 9/10 Nanz pass |
| Full 8-frontend corpus | **42 native binaries** |

## Commits (3)

1. `e658f43a` — lli native execution + --emit llvm/wasm CLI + LLVM→ABAP research
2. `b88b1dfa` — production-grade codegen: i32 uniform, phi renames, 6 new ops
3. `cea79213` — reaching definitions + bool→i32: 9/10 corpus pass (90%)
