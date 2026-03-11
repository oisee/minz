# Frontend & Backend Diagnostic — 2026-03-11

> Honest state of every frontend, backend, and tool in the MinZ compiler system.
> Based on `go test ./...` run against v0.19.5+ (commit 4d34355).

---

## Test Results by Package

| Package | Status | Notes |
|---|---|---|
| `pkg/nanz` | ✅ GREEN | Nanz parser — all tests pass |
| `pkg/hir` | ✅ GREEN | HIR + lowering — all pass |
| `pkg/plm` | ✅ GREEN | PL/M-80 frontend — all pass |
| `pkg/mir2` | ✅ GREEN | **160 tests** — allocator, codegen, optimizations, assertions |
| `pkg/mir2c` | ✅ GREEN | MIR2→C99 backend — E2E roundtrip via gcc |
| `pkg/mir2qbe` | ✅ GREEN | MIR2→QBE backend — E2E roundtrip via qbe |
| `pkg/qbe2mir2` | ✅ GREEN | QBE→MIR2 parser — round-trip tests |
| `pkg/z80asm` + `regression` | ✅ GREEN | MZA assembler — all pass |
| `pkg/emulator` | ✅ GREEN | MZE Z80 emulator — 1335/1335 FUSE |
| `pkg/z80testing` | ✅ GREEN | E2E harness — all pass |
| `pkg/z80timing` | ✅ GREEN | T-state timing tests — all pass |
| `pkg/disasm` + `analysis` | ✅ GREEN | MZD disassembler — all pass |
| `pkg/ir`, `interpreter`, `optimizer`, `semantic` | ✅ GREEN | Old MinZ path — all pass |
| `pkg/parser`, `parser/participle` | ✅ GREEN | Old MinZ participle parser — all pass |
| `pkg/spectrum` + `formats` | ✅ GREEN | MZX ZX Spectrum emulator — all pass |
| `pkg/mirvm`, `pkg/tas`, `pkg/trace` | ✅ GREEN | MZV runner and utilities — all pass |
| `tests/iterator_corpus` | ✅ GREEN | 11/11 iterator chain E2E — all pass |
| **`tests/`** | 🔴 2 FAIL | `TestFeature_UFCS_MethodCall`, `TestFeature_InlineAsm` |
| **`pkg/codegen`** | 🔴 VET FAIL | `go vet` blocks test run — non-constant format strings |
| **`cmd/repl`** | 🔴 BUILD FAIL | `int` vs `uint16` type mismatch from z80asm API change |

**Overall: 25 packages green, 3 red.**

---

## Frontends

### Nanz (`.nanz`) — ✅ Production, Active

The main frontend. All tests pass.

- **Parser**: native Go (participle-based), zero external deps
- **Pipeline**: Nanz → HIR → MIR2 → Z80 (+ MIR2→C / MIR2→QBE)
- **Features**: structs, methods, UFCS, interfaces, lambdas, closures, iterators,
  `range(lo..hi)`, operator overloading, ranged types `u8<lo..hi>`, LUT synthesis,
  dual-VM assertions, `@extern`, `@smc`, register annotations (`@z80_hl` etc.)
- **Known language gaps** (tracked in `docs/Open_Bugs_RCA.md`):
  - 🔴 BUG-003: `ptr[i]` inside while loop → invalid Z80 (`EX DE,HL` / `ADD F,DE`)
  - 🔴 BUG-006: Zero-size struct globals not emitted → undefined symbol at link time
  - 🟡 BUG-004: Non-zero-lo LUT + contract opt → class mismatch (unit tests bypass it)
  - 🟡 BUG-005: `applySubSwapNeg` missing u16 guard (one-line fix, workaround exists)

### PL/M-80 (`pkg/plm`) — ✅ 100% Corpus, Active

26/26 Intel 80 Tools source files parse. 1338 functions / 943 globals / 11661 statements lowered to HIR.

- **Pipeline**: PL/M-80 → HIR → MIR2 → Z80 (E2E wired in `cmd/minzc/main.go`)
- **Preprocessor**: LITERALLY macro chains, `$INCLUDE` with CP/M device designators, binary literals, escaped `''` quotes
- **Known gaps**:
  - `STRUCTURE` type (nested records)
  - `PUBLIC` / `EXTERNAL` (multi-module linking)
  - `INPUT(port)` / `OUTPUT(port)` hardware I/O intrinsics
  - 16-bit comparisons (parsed, incomplete codegen)

### MinZ (`.minz`, `pkg/parser/participle`) — ⚠️ Frozen

The original frontend. Not being developed — only maintained.

- Compiles the majority of existing `.minz` examples
- 2 failing integration tests:
  - `TestFeature_UFCS_MethodCall`: typed pointer receiver `fun Foo_method(self: *Foo)` not in old grammar
  - `TestFeature_InlineAsm`: `@asm { ... }` block not parsed by old grammar
- **These are not regressions** — features that only exist in the Nanz frontend.
  Tests should be moved to a Nanz test suite or marked `t.Skip`.
- The old pipeline (`pkg/ir` → `pkg/codegen`) is still available but not developed.

---

## Backends

### Z80 via MIR2 (`pkg/mir2/z80codegen.go`) — ✅ Production

The active backend. 160 tests, all green.

Full optimisation pipeline (in order):
1. **LUTGen** — pure ranged functions → compile-time lookup tables
2. **EliminateDeadBlocks** → **ReorderBlocks**
3. **Constant pipeline** to fixpoint: PropagateConstants → FoldConstants → SimplifyIdentities → ConstantCallElim
4. **DeadStoreElim**
5. **BranchEquiv** — VM-based proof of redundant conditional branches
6. **CondRetSink** → **hoistReorderSubBeforeCmp** → **CmpSubCarry**
7. **MIR2 Verify** — structural correctness check
8. **Phase 5b: OptimizeContracts** — interprocedural calling convention DP
9. **PBQP allocator** — cost-weighted register assignment
10. **Post-allocation copy coalescing**
11. **Dual-VM assertions** — MIR2 VM before codegen, Z80 emulator after
12. **Z80Codegen** — `genMul16` strength reduction, IX/IY addressing, peephole

Open bugs:
- 🔴 BUG-003: `ptr[i]` in while loop — `PtrAdd` cycle produces invalid Z80
- 🔴 BUG-006: Zero-size struct globals silently omitted → `undefined symbol`
- 🟡 BUG-001: GCD parallel-copy bloat (PBQP affinity edges not yet added)
- 🟡 BUG-002: `forEach` constant rematerialization in parallel-copy resolver

### MIR2→C (`pkg/mir2c`) — ✅ Correctness Oracle, Working

Translates MIR2 IR to C99. Compiled and executed via `gcc`.

**Why it exists**: semantic cross-checking. Any function that produces a wrong answer
in Z80 binary but the right answer in the C translation has a codegen bug.
Also useful for portability — the same MIR2 module runs on any platform with a C compiler.

E2E tests passing: `abs_diff`, `fib`, `clamp`, `max3` (PL/M + Nanz sources).

### MIR2→QBE (`pkg/mir2qbe`) — ✅ Modern Target Explorer, Working

Translates MIR2 IR to [QBE](https://c9x.me/compile/) intermediate representation.
QBE compiles to native amd64 / arm64 / riscv64.

**Why it exists**:
1. **SSA validation**: QBE is strict about SSA form. If MIR2→QBE produces valid QBE,
   the MIR2 module is structurally sound.
2. **Modern targets**: The same frontend (Nanz or PL/M) can compile to x86-64 with zero
   extra work. Useful for testing on the host machine.
3. **E2E semantic testing**: Run the compiled binary and check the answer — faster than the Z80 emulator.

E2E tests passing: `abs_diff`, `clamp`, `gcd`, UFCS dispatch, zero-cost interfaces (PLM + Nanz).

### QBE→MIR2 (`pkg/qbe2mir2`) — ✅ Parser, Working

Parses QBE IR text into MIR2 data structures. Enables "QBE as a frontend" experiments
(e.g. a Clang-targeting-QBE pipeline that feeds into the Z80 backend).

### Old Multi-Backend (`pkg/codegen`) — 🔴 Vet-Blocked, Experimental

Contains the old backends for the `.minz` frontend path via `pkg/ir`.
`go build ./pkg/codegen/` works fine. `go test ./pkg/codegen/` fails on vet.

**Vet errors**: ~20 `non-constant format string in call to emit/Fprintf` in:
- `m6502_gen.go` (6502/NES)
- `gb.go` (Game Boy)
- `i8080.go` (Intel 8080)
- `m68k.go` (Motorola 68000)

Status per backend:

| File | Target | State |
|---|---|---|
| `z80.go` | Z80 (old) | Partial; superseded by MIR2 path |
| `c.go` / `c_backend.go` | C99 | Partial; superseded by `pkg/mir2c` |
| `i8080.go` | Intel 8080 | Experimental, untested |
| `gb.go` | Game Boy (LR35902) | Experimental, untested |
| `m6502_gen.go` | 6502/NES | Experimental |
| `m68k.go` | MC68000 | Experimental |
| `crystal.go` | Crystal lang | Stub/broken |
| `llvm_backend.go` | LLVM IR | Stub/broken |
| `wasm_backend.go` | WebAssembly | Stub/broken |

None of these are wired into the active pipeline. They are reached only via the frozen
`.minz` → `pkg/ir` → `pkg/codegen` path.

---

## Tools

| Tool | Status | Notes |
|---|---|---|
| **MZC** (`cmd/minzc`) | ✅ Production | Main compiler — `.nanz`, `.plm`, `.minz`, `.hir` |
| **MZA** (`pkg/z80asm`) | ✅ Production | Z80 assembler, all tests green |
| **MZE** (`pkg/emulator`) | ✅ Production | 1335/1335 FUSE tests |
| **MZX** (`pkg/spectrum`) | ✅ Done | ZX Spectrum emulator, T-state accurate |
| **MZD** (`pkg/disasm`) | ✅ Done | Disassembler + analysis engine |
| **MZLSP** (`pkg/lsp`) | ✅ Done | LSP — diagnostics, hover, goto-def, completion. **0 tests** |
| **MZV** (`cmd/mzv`) | ✅ Done | MIR VM runner — breakpoints, tracing, PNG export |
| **MZRUN** (`cmd/mzrun`) | ✅ Done | Remote runner (DZRP protocol) |
| **MZTAP** (`cmd/mztap`) | ✅ Done | TAP file loader |
| **MZR** (`cmd/repl`) | 🔴 Build fail | `int`/`uint16` mismatch; also broken semantically (`:run` unimplemented) |
| **DAP** (`pkg/dap`) | 📋 Not started | Debug Adapter Protocol |

---

## What Actually Needs Fixing

### Must fix (blocking `go test ./...` from going fully green)

**1. `cmd/repl` build** — trivial type cast fix

In `cmd/repl/compiler.go:198` and `:205`:
```go
result.EntryPoint = uint16(asmResult.Origin)   // was: asmResult.Origin (int)
result.Functions[name] = uint16(addr)          // was: addr (int)
```
5-minute fix. Note: the REPL is still functionally broken after this (`:run` unimplemented),
but at least `go test ./...` won't show a build failure.

**2. `pkg/codegen` vet errors** — non-constant format strings

In `m6502_gen.go`, `gb.go`, `i8080.go`, `m68k.go`: the `emit()` helper takes a format string
as a variable, which `go vet` flags as potentially unsafe. Fix options:
- Convert `emit(str)` → `emit("%s", str)` at the call sites (safest)
- Or add `//nolint:govet` to the file headers (acceptable since these backends are experimental)

**3. `tests/` UFCS + InlineAsm** — wrong frontend, not regressions

These tests send Nanz-syntax code to the old MinZ participle parser. They test features
that the old parser never supported and never will. Fix options:
- Add `t.Skip("requires Nanz frontend — use pkg/nanz tests")` to both
- Or delete and add equivalent tests to `pkg/nanz/nanz_test.go`

### Should fix (open bugs, severity-ordered)

| Priority | Bug | Description | Effort |
|---|---|---|---|
| 🔴 High | BUG-003 | `ptr[i]` in while loop → invalid Z80 | Medium |
| 🔴 High | BUG-006 | Zero-size struct globals not emitted | Small |
| 🟡 Medium | BUG-001 | GCD parallel-copy bloat (PBQP affinity) | Medium |
| 🟡 Medium | BUG-002 | `forEach` constant rematerialization | Small |
| 🟡 Low | BUG-005 | `applySubSwapNeg` missing u16 guard | Tiny (one line) |

### Nice to have

- **`pkg/pipeline` tests**: The dual-VM assertion infrastructure, LUTGen wiring, and full
  Nanz→Z80 chain have no package-level tests. A basic smoke test would prevent silent
  regressions.
- **MZLSP tests**: The LSP server has 0 tests. At minimum: parse a file with a diagnostic
  and verify the `Diagnostics` list.
- **BUG-004** (non-zero-lo LUT + contract opt): Needs pipeline ordering fix — LUTGen must
  run after contract opt, or contract opt must be LUT-aware.

### Not touching

- Old `pkg/codegen` experimental backends (GB, 6502, M68K, Crystal, WASM, LLVM):
  experimental, not wired to anything, vet fix is sufficient.
- `pkg/dap`: Not started, not needed until core language is stable.
- `pkg/hirwat`, `pkg/meta`, `pkg/metafunction`: Internal utilities, no known issues.

---

## Summary

The **active pipeline** (Nanz / PL/M → HIR → MIR2 → Z80) is healthy: 25 of 28 test
packages are green, 160+ MIR2 tests pass, and the full optimisation chain (PBQP, LUTGen,
dual-VM assertions, CondRetSink, BranchEquiv, multiply strength reduction) is wired and
verified. The 3 red packages are all in **legacy / experimental territory** and have
trivially small fixes — none touch the active pipeline.

The two blocking open bugs (BUG-003, BUG-006) are scoped to specific language patterns
and do not affect the majority of programs.
