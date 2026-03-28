# CLAUDE.md

This file provides guidance to Claude Code when working with the MinZ compiler repository.

---

## 🚨 CURRENT PRIORITIES

### 1. Register Allocator & Loop Codegen Bugs
**Status:** Known, not yet fixed. Blocks complex programs.
- While/for loops: register allocator overwrites operands (same phys reg for two live virtuals)
- `loadToHL` uses stale HL values in multi-expression contexts
- Loop rerolling too aggressive across function call boundaries
- See `docs/adr/` for ADR-0006, ADR-0007

### 2. Iterator Chain Fusion
**Status:** Pipeline correct (11/11 E2E), fusion optimizer live. ~5x perf overhead from memory-backed registers.
- 11/11 E2E hex-verified: forEach, take, skip, map, filter, lambda map/filter, multi-stage chains
- Fusion optimizer inlines small callbacks into DJNZ loops (eliminates CALL/RET)
- **Broken on Z80:** enumerate (B=counter+index conflict), reduce (A overwritten by 2nd SMC param)
- **Bottleneck:** Register allocator puts all virtuals through $F0xx memory (~207T actual vs ~43T ideal per element)
- 87+ tests across 7 layers
- See [Iterator Implementation Status](docs/Iterator_Implementation_Status.md)

### 3. MIR2 Open Bugs
**Status:** 8 tracked bugs (4 fixed, 4 open — 1 blocking 🔴). See **[docs/Open_Bugs_RCA.md](docs/Open_Bugs_RCA.md)** for full RCA.
- 🔴 **BUG-008** Arena codegen: impossible `LD IXL, (IX+d)` + self-pointer loss (blocks struct methods)
- 🟡 **BUG-001** GCD parallel-copy bloat + `$0000` ROM spills (PBQP affinity / spill relocation)
- 🟡 **BUG-006** Zero-size struct globals not emitted (undefined symbol at link time)
- 🟡 **BUG-007** Spurious adapter LD when caller/callee share PFCCO convention
- ✅ **BUG-002** forEach constant rematerialization — fixed 2026-03-12
- ✅ **BUG-003** `ptr[i]` in while loop — fixed 2026-03-12
- ✅ **BUG-004** Non-zero-lo LUT pipeline ordering — fixed 2026-03-12
- ✅ **BUG-005** `applySubSwapNeg` u16 guard — fixed 2026-03-12

### 4. LIR Backend (Guided PBQP+WFC Register Allocation)
**Status:** 🚧 Production-matching codegen for leaf functions, 94.6% C89 corpus convergence.
- **Branch:** `feat/lir-backend` (24+ commits, ~7500 LOC)
- **Pipeline:** MIR2 → Bridge → Combine(ISLE) → isel(PatternTable) → WFC(PBQP-guided) → peephole → emit
- **PBQP→WFC synthesis:** PBQP provides global allocation hints, WFC enforces Z80-specific constraints. Output matches production codegen for leaf functions.
- **Call support:** OpCall lowering with arg setup moves, `DstAllowed` class constraints, tail call opt (CALL+RET → JP, saves 17T)
- **IXH/IXL L2 spill:** Undocumented IX/IY half-registers as call-safe storage (8T). 4 bytes of fast spill without touching stack. No existing Z80 compiler uses this.
- **WFC passes:** forward, backward, vregConsistency, clobberPass (call-safe narrowing), Collapse with `pickPreferred(hints)`
- **Save-before-overwrite:** Bridge-level insertion of save moves for vregs at risk from destructive ALU ops (Z80 accumulator architecture)
- **Peephole:** LD r,r no-op elimination, tail call CALL+RET→JP
- **ISLE combining:** load16_le fusion (FatFS ld_word: 8→2 ops), MUL strength reduction
- **WFC Dimension 2:** Inter-block constraint propagation across CFG edges — `ProgWFC` with RPO collapse, back-edge fixpoint
- **Z80 descriptor:** 22 locs (GPR + IX/IY halves + spill), 41+ patterns, DD/FD prefix rules
- **Corpus:** **948/948 pipeline completion** (100%); VM-verified: **97.9%** C89 on risc32 (15 struct/fptr divergences)
- **Runtime:** `__mul8` (A×B→A, ~80T), `__mul16` (HL×DE→HL, ~200T) — shared routines, emitted once per module
- **Remaining:** EXX shadow regs (L3), ISLE const-MUL reduction, production switch as default `--lir`
- See [Architecture](docs/LIR_Backend_Architecture.md), [Reference](docs/LIR_Backend_Reference.md), [Report 094](reports/2026-03-18-094-LIR-100-Percent-C89-Corpus.md), [ADR-0033](docs/adr/0033-lir-pipeline-integration.md)

### 5. VIR Backend (Z3 Unified Solver — `--vir`)
**Status:** ✅ Non-leaf production, O(1) regalloc for 91% corpus, -60% vs SDCC. ~12K LOC.
- **Pipeline:** MIR2 → Bridge → ISLE → Z3-PFCCO(Module) → **O(1) Enriched Table** / Z3-CFG(Function) → Grace(PIR) → Peephole → Z80 ASM
- **Package:** `pkg/vir/` — vir.go, z80.go, bridge.go, solver.go, cfgsolver.go, pfcco.go, isle.go, pipeline.go, regalloc_table.go
- **Corpus:** 820 functions (546 Nanz + 274 C89), 235 unique signatures. Non-leaf functions verified (Hello Frill, Tetris CP/M).
- **O(1) Regalloc (NEW):** 5-level pipeline replaces Z3 30s solve:
  - L0: Tarjan cut vertex decomposition → free split (91% with enriched)
  - L1: Enriched table ≤6v → hash lookup O(1) (79% corpus, 37.6M precomputed entries)
  - L2: EXX bipartite check → dual-bank scheduling (70% functions feasible)
  - L3: GPU min-cut partition → <1ms for ≤14v (z80-optimizer CUDA kernel)
  - L4: Z3 fallback (<1% functions)
- **Z3-PFCCO:** Module-level calling convention optimization — Z3 considers all call sites simultaneously. The SDCC killer.
- **Per-instruction variables:** `lv{vreg}_i{inst}` — solver plans moves as part of optimal solution, not as post-pass fixup.
- **CFG-aware encoding:** Per-block variables + edge constraints. Handles conditionals/loops correctly.
- **Inline runtime:** div8/mod8/mul8 per call site with Z3-chosen registers, no fixed ABI overhead.
- **IXH/IXL spill:** Undocumented half-index regs as call-safe storage (8T vs 11T PUSH/POP).
- **ISLE combining:** load16_le fusion (FatFS ld_word: 5 insts vs SDCC 29B), identity/strength reduction.
- **Grace PIR:** Dead register before RET, EX DE,HL elimination, add-zero removal.
- **OpAsmBlock:** Inline asm as opaque black box with clobbers — surrounding code fully Z3-optimized.

#### VIR Design Wisdom (hard-won lessons)
1. **Optimal codegen over compile speed:** Compilation time is not a constraint — even minutes per function is acceptable. Z3 can be retried with different seeds, multiple strategies explored, `(minimize)` always preferred. We are here to bring **optimal solutions**, not fast compilation.
2. **Reliability first:** Understand compilation path before fixing quality. Add tracing/audit before changing codegen. Run full corpus after any change.
3. **Hard vs soft Z3 constraints:** Hard `(assert ...)` for facts (ABI, spill tiers, return convention). Soft `(ite cost)` for hints (SrcHint, register preferences). Hard hints that conflict with tied-dst patterns → unsat.
4. **Per-instruction vs global variables:** Global (one `loc_v` per vreg) is fast but fails at >7 live vregs. Per-instruction (`lv{v}_i{k}`) handles everything. Use global as Phase 1, per-inst as Phase 2.
5. **Inline runtime per call site:** Don't pin runtime ABI — emit div/mul loop body inline with Z3-chosen registers. Deduplicate identical instances post-emit.
6. **Pre-solver pass interaction:** Pre-tie moves rewrite `v1` → `v1_copy`. DON'T propagate param constraints to copy vregs (both live at move, both constrained to same reg → unsat). Constrain originals only.
7. **Z3 minimize pitfall:** `(minimize total_cost)` uses opt module, not SAT. On >100 ITE vars it returns "unknown". Use plain `(check-sat)` for large problems — correct, just not provably optimal.
8. **Dual-mode: constrained vs standalone+adapter:** Solve each function twice — with ABI constraints and without. Pick the cheaper option. Standalone winner gets adapter LD moves at entry/exit. Benchmark shows 7 funcs / 47 insts saved across Nanz corpus (e.g., `_dec`: 32→1 insts).
9. **O(1) before Z3:** Compute (interference_shape, op_bag) signature → enriched table lookup. 91% of real functions hit. Only fall to Z3 on miss. Compile time: 30s → 0ms for majority.
10. **Cross-block params need injection:** Per-block Z3 liveness misses function params used in later blocks. Inject at ALL entry-block instructions + recompute blockLiveIn. Without this, edge moves for params are silently lost.
11. **Never remove LD before CALL:** Peephole "LD A,X / CALL → dead" is WRONG — the LD may be loading the callee's argument. The solver only emits needed moves.

#### VIR Known Issues / Roadmap
- ✅ ~~abs_diff CFG solver unsat~~ — now 4 insts (optimal SUB/RET NC/NEG/RET), CFG solver succeeds
- ✅ ~~fib parallel-copy~~ — now 12 insts (45% win vs SDCC), tight loop body
- ✅ ~~Non-deterministic coalescing~~ — sort tie-break by vreg ID applied, deterministic
- ✅ ~~CALL arg setup~~ — 4 bugs fixed (vreg collision, DstHint, peephole, coalescing). Non-leaf production.
- ✅ ~~Cross-block params~~ — param injection + pair→half move fallback. bool_and/is_digit correct.
- ✅ ~~Loop head labels~~ — emitPIRWithLabels fixed for 0-op entry blocks + island dedup.
- **Grace on PIR** — dead register elimination across call boundaries, block merging, fallthrough optimization
- **RLD/RRD for 4-bit shifts** — Z80 digit-rotate instructions for nibble packing
- **ISLE store16_le via DE** — EX DE,HL bracket pattern
- **EX DE,HL elimination** — track register preferences across calls via Z3-PFCCO
- **Inline 16-bit div/mul** — currently falls back to shared routine; should inline per call site like 8-bit

### 6. Rewrite Triad (ISLE + Grace + Datalog)
**Status:** ✅ Wired into pipeline, 94.1% convergence. See [Report 097](reports/2026-03-19-097-Rewrite-Triad-Infrastructure.md).
- **Package:** `pkg/rewrite/` — zero imports from hir/mir2/lir, operates on abstract `IRGraph`/`IRGraphMut` interfaces
- **ISLE:** Term rewriting with guards `(if (< ?n 256))` + extern Go callbacks. 542 LOC.
- **Grace:** Graph pattern matching on CFG (Cypher-equivalent). 1255 LOC, 13 tests across 7 Cypher categories.
- **Datalog:** Fact database with wildcard queries. 101 LOC.
- **Pipeline integration:** `Options{UseGrace: true}` enables Grace path. 5 rules (DSE, CondRetSink, SplitJoinRet, DeadBlockArgElim, FuseAbsDiff) with custom Go actions. 370 LOC runner.
- **Convergence:** 17/17 pass verification (100%), 16/17 byte-identical assembly (94.1%). 89 funcs processed, 63 rewrites applied.
- **Stats:** `GraceStats` tracks per-rule fire counts across module compilation.
- **NOT a Z80 constraint solver:** Grace handles CFG patterns. Z80 register constraints / reject+backtrack is done by WFC in LIR (§4 above).

### 7. LSP / DAP / Developer Tooling
**Status:** Not started. Planned after core language stability.

---

## 🎓 Quick Start for AI Colleagues

- **[MinZ Crash Course for AI Colleagues](AI_COLLEAGUES_MINZ_CRASH_COURSE.md)** - Complete training
- **[GenPlan.md](docs/GenPlan.md)** - Development plan & roadmap
- **[Open Bugs & RCA](docs/Open_Bugs_RCA.md)** - Known issues with root cause analysis

## 🏗️ Architecture References

- **[LIR_Backend_Architecture.md](docs/LIR_Backend_Architecture.md)** - PBQP+WFC+ISLE: how three solvers cooperate
- **[LIR_Backend_Reference.md](docs/LIR_Backend_Reference.md)** - Quick reference: pipeline, DSLs, data structures
- **VIR Backend** (`pkg/vir/`) - Z3 unified solver: joint isel+regalloc, PFCCO, inline runtime — see §5 in Current Priorities
- **[INTERNAL_ARCHITECTURE.md](minzc/docs/INTERNAL_ARCHITECTURE.md)** - Complete compiler internals
- **[COMPILER_SNAPSHOT.md](COMPILER_SNAPSHOT.md)** - Current state tracking
- **[149_World_Class_Multi_Level_Optimization_Guide.md](docs/149_World_Class_Multi_Level_Optimization_Guide.md)** - Revolutionary optimization strategy
- **[Report 097: Rewrite Triad](reports/2026-03-19-097-Rewrite-Triad-Infrastructure.md)** - ISLE+Grace+Datalog declarative optimization engines (infrastructure, not yet in pipeline)
- **[Report 111: GPU Regalloc Table](reports/2026-03-24-111-GPU-Precomputed-Regalloc-Table.md)** - Precomputed register allocation from CUDA brute-force (61 entries, O(1) lookup)
- **[GPU Bruteforce Roadmap](../z80-optimizer/BRUTEFORCE_ROADMAP.md)** - Beyond peephole: constant multiply, division, screen address, sin/cos — all via exhaustive GPU search

## 🎯 Custom Commands

### Core Development
- `/upd` - Update all documentation
- `/release` - Prepare new release
- `/test-all` - Comprehensive test suite
- `/benchmark` - Performance benchmarks
- `/inbox` - Process articles from inbox/ to docs/

### AI Orchestration  
- `/ai-testing-revolution` - Build testing infrastructure
- `/parallel-development` - Execute multiple tasks
- `/performance-verification` - Verify optimization claims

### Fun Commands 🎉
- `/cuteify` - Add emojis and fun
- `/celebrate` - Achievement recognition

## 🛠️ Development Tools & Status (v0.22.0)

### Self-Contained Toolchain
- **MZC** - MinZ Compiler (Go, ~90K LOC)
- **MZA** - Z80 Assembler (table-driven, `[addr]` bracket syntax)
- **MZE** - Z80 Emulator (100% coverage via remogatto/z80)
- **MZX** - ZX Spectrum Emulator (T-state accurate, AY sound, Ebitengine)
- **MZD** - Z80 Disassembler (IDA-like analysis, ABI propagation, register tracking)
- **MZR** - Interactive REPL
- **MZRUN** - Remote runner (DZRP protocol)

---

## 📊 Feature Status Legend

| Tag | Meaning |
|-----|---------|
| ✅ **DONE** | Working in production |
| 🚧 **WIP** | In active development |
| 📋 **TOBE** | Planned for implementation |
| ⏸️ **PARKED** | Deferred, may return later |
| ❌ **REJECTED** | Will NOT be implemented |

---

## ✅ DONE (Working Features)

### Core Language
- Types: `u8`, `u16`, `i8`, `i16`, `bool`, `void`
- Functions: `fun`/`fn` declaration, multiple returns, nested functions
- Control flow: `if`/`else`, `while`, `for i in 0..n`
- Structs: declaration and field access
- Enums: `enum State { IDLE, RUNNING }` with values, `State.IDLE` dot syntax
- Type aliases: `type PlayerID = u8` — structural, zero-cost
- Arrays: declaration, indexing (literals need optimization)
- Global variables: `global` keyword
- Function overloading: multiple signatures
- **Native parser**: Participle-based (pure Go, replaced tree-sitter Feb 2026)

### Advanced Features
- **Ruby interpolation**: `"Hello #{name}!"` ✅
- **UFCS**: `obj.method()` via zero-cost interfaces ✅
- **Lambdas**: Full closure support, zero-cost ✅
- **TRUE SMC**: Self-modifying code optimization ✅
- **CTIE**: Compile-Time Interface Execution (trait monomorphization) ✅
- **@extern FFI**: Call external ROM/BIOS functions ✅
- **RST optimization**: Auto-convert to RST instructions ✅
- **Operator overloading**: Custom operators for types ✅

### Metafunctions
- `@define("template", args)` - Text substitution ✅
- `@print` - Optimized string output ✅
- `@if/@elif/@else` - Conditional compilation ✅
- `@error` - Error propagation with CY flag ✅

### Tooling
- Error messages with file:line:col format ✅
- Multi-backend: Z80 (production), C (partial), i8080/M68k (untested), 6502/GB/WASM/LLVM/Crystal (stubs/broken) ✅
- 100% Z80 instruction coverage in emulator ✅
- **Compilation provenance tracing**: per-function ASM annotations (backend, passes, splits, fallbacks) + module summary + label audit ✅

---

## 🚧 WIP (In Development)

- **LIR backend**: PBQP→WFC guided regalloc, production-matching leaf codegen, IXH/IXL spill, tail call opt (see §4 above, `feat/lir-backend`)
- **Iterator chain fusion**: 11/11 E2E correct, fusion optimizer live, ~5x perf overhead (register allocator bottleneck)
- **Pattern matching**: Syntax parses, codegen partial
- **@minz[[[...]]]**: Limited compile-time execution
- **MIR VM**: Arrays/structs working (mirvm package, MZV runner works)
- **Array literal optimization**: IR skeleton exists, codegen not yet
- **Rewrite Triad**: ISLE+Grace+Datalog in `pkg/rewrite/` — wired into pipeline (`UseGrace: true`), 94.1% convergence with Go originals on Nanz corpus. 63 rewrites across 89 functions. See §5, [Report 097](reports/2026-03-19-097-Rewrite-Triad-Infrastructure.md)
- **MZR REPL**: ❌ Broken — `compileModule()` returns empty module, `:run` unimplemented

---

## 📋 TOBE (Planned)

- **DAP debugger** - Step-through debugging (native, beyond DeZog)
- **MZR REPL** - Fix compilation pipeline (semantic analysis not wired)
- **WASM playground** - Online demo
- **Generator syntax** - `gen`/`yield` for lazy iteration

---

## ⏸️ PARKED (Deferred)

- **Generics `<T>`** - Use function overloading instead
- **Option/Result types** - Use `@error` pattern instead
- **`?` operator** - Use explicit error checking

---

## ❌ REJECTED (Won't Implement)

- **C++ style templates** - Too complex for Z80 target
- **Multiple inheritance** - Use interfaces instead
- **Garbage collection** - Manual memory for retro targets
- **Exceptions** - Use `@error` with CY flag instead
- **Runtime reflection** - No runtime overhead allowed
- **Dynamic dispatch vtables** - Zero-cost interfaces only

## 🎯 Metafunction Design Decisions

**CRITICAL:** These are settled design decisions - do not confuse them!

- **@minz[[[...]]]** - Immediate compile-time execution
  - Takes NO ARGUMENTS (not a template!)
  - Uses @emit() to generate code line by line
  - Example: `@minz[[[ @emit("fun foo() -> void {}") ]]]`

- **@define("template", args...)** - Preprocessor macro substitution
  - Processed BEFORE parsing (pure text replacement)
  - Uses {0}, {1} placeholders for arguments
  - Example: `@define("fun {0}() -> {1}", "getName", "str")`
  - Status: ✅ FULLY IMPLEMENTED AND WORKING!

- **@lua[[[...]]]** - Lua compile-time execution
  - Full Lua scripting for complex metaprogramming
  - Has emit() function for code generation

See `docs/Metafunction_Design_Decisions.md` for complete details.

## 🚀 TSMC: Revolutionary Paradigm

**True Self-Modifying Code** - Programs rewrite themselves for optimization:
- **Smart Patching**: Single-byte opcode changes (7-20 T-states vs 44+)
- **Parameter Injection**: Values patched into instruction immediates
- **Behavioral Morphing**: One function, infinite behaviors
- Complete docs: `docs/145_TSMC_Complete_Philosophy.md`

### TSMC Tunnels (Register Preservation Across CALLs)

TSMC tunnels save/restore register values across CALL instructions by patching the immediate byte of a subsequent LD instruction:

```z80
; 8-bit: save A across CALL (20T total, SP untouched)
LD (.tsmc_n+1), A      ; 13T — patch the NN byte below
CALL some_function      ; clobbers A
.tsmc_n:
LD A, 0                 ; 7T — 0 was overwritten! Reads saved value as immediate

; 16-bit: save DE across CALL via patching LD DE,NNNN (44T total)
LD A, E / LD (.tsmc+1), A / LD A, D / LD (.tsmc+2), A  ; 34T save
CALL func
.tsmc: LD DE, 0000      ; 10T — both bytes patched

; NOT recursion-safe: nested call overwrites the patched byte!
```

**Spill tier hierarchy for VIR Z3 solver:**
- **L0: Primary regs (A-L)** — 0T, 7 slots
- **L1: IXH/IXL/IYH/IYL** — 8T, 4 slots, callee-saved, always safe
- **L2: I register** — 18T, 1 slot, clobbers P/V flag. **UNSAFE in IM 2** (I = interrupt vector table high byte). Safe in IM 0/1 or with DI/EI bracket. CP/M: usually safe. ZX Spectrum IM 2 demos: NEVER touch I.
- **L2b: R register** — 18T, 1 slot, R[7] preserved, R[6:0] auto-increments (recoverable if compiler knows instruction count N between save/restore)
- **L3: TSMC 8-bit tunnel** — 20T, unlimited, safe when no recursion between endpoints
- **L3b: Shadow regs (EXX/EX AF,AF')** — 4T batch swap, 7+7 slots, ALL swap simultaneously
- **L4: PUSH/POP pairs** — 21T, unlimited, always safe, @error needs N×POP cleanup
- **L5: Memory spill** — 26T, unlimited, always safe

**TSMC wins for 8-bit** (20T vs PUSH/POP 21T + saves whole pair). **PUSH/POP wins for 16-bit pairs** (21T vs TSMC 44T). For @error propagation: PUSH/POP + compiler-generated stack cleanup (N×POP on error path) is cheaper than TSMC for pairs.

## 🏆 Zero-Cost Abstractions on Z80

### ✅ Zero-Cost Lambda Iterators (v0.10.0) 🎊
```minz
numbers.iter()
    .map(|x| x * 2)
    .filter(|x| x > 5)
    .forEach(|x| print_u8(x));
```
**Revolutionary**: Lambda-to-function transform with DJNZ optimization!

### ✅ Zero-Cost Interfaces
```minz
circle.draw()  // Direct CALL Circle_draw - NO vtables!
```

### ✅ Zero-Overhead Lambdas
```minz
let add = |x: u8, y: u8| => u8 { x + y };
add(5, 3)  // Direct CALL - 100% performance
```

## 📚 Standard Library (v0.15.0+)

MinZ includes a comprehensive stdlib optimized for Z80/retro systems:

| Module | Description |
|--------|-------------|
| `math/fast` | Sin/cos/sqrt lookup tables (256 entries each) |
| `math/random` | LFSR PRNG, noise functions, probability helpers |
| `graphics/screen` | Pixel/line/circle/rectangle (ZX Spectrum optimized) |
| `input/keyboard` | Keyboard matrix reading, debouncing, game helpers |
| `text/string` | strlen, strcmp, strcpy, strcat, trim, etc. |
| `text/format` | Number to string (decimal, hex, binary) |
| `sound/beep` | Beeper SFX (click, buzz, jump, explosion) |
| `time/delay` | Frame timing, delays, animation helpers |
| `mem/copy` | Fast memcpy/memset/memcmp using LDIR |
| `cpm/bdos` | CP/M BDOS system calls |
| `tui/render` | VT100 TUI primitives (cursor, color, input, box drawing) |
| `sql/sqlite` | Z80 SQLite bridge via I/O ports ($41/$43/$45/$47) |

### Usage Example
```minz
import stdlib.graphics.screen;
import stdlib.input.keyboard;
import stdlib.time.delay;

fun main() -> void {
    clear_screen();
    draw_circle(128, 96, 50);

    loop {
        wait_frame();
        let dx = get_key_dx();
        // Move sprite based on input...
    }
}
```

## 📋 Development Commands

### Cross-Session Communication (dedelulu)
Multiple Claude Code sessions can communicate via dedelulu:
```bash
# Discover all active sessions
dedelulu explore

# Send a message to another session
dedelulu send <session_id>:main "your message here"

# Example: send to the main minz repo session
dedelulu send bazyqg2s:main "Hey! VIR solver results for screen_alv.nanz..."
```
Sessions are identified by `session_id:worker_name`. Use `dedelulu explore` to find live sessions.
Messages appear as `[from:main]` in the recipient's conversation. Reply with `dedelulu send`.

Active project sessions:
- `~/dev/minz` — main MinZ compiler (production codegen, @screen, ABAP)
- `~/dev/minz-vir` — VIR backend (Z3 solver, dual-mode, adapters)
- `~/dev/z80-optimizer` — CUDA superoptimizer (602K rules, GPU regalloc)
- `~/dev/dedelulu` — cross-session messaging tool itself

### Releasing
**IMPORTANT:** Before creating a release, ALWAYS check the latest version on GitHub:
```bash
gh release list --repo oisee/minz --limit 3
```
Bump from the latest GitHub version, not from what CLAUDE.md says (it may be stale).

### Build & Test
```bash
# Build compiler
cd minzc && make build

# Test all examples
./compile_all_examples.sh

# Compile with optimizations
./minzc program.minz -O --enable-smc
```

### Multi-Backend Compilation
```bash
mz program.minz -b z80 -o program.a80       # Z80 (default, production)
mz program.minz -b c -o program.c           # C99 (testing)
mz program.minz -b crystal -o program.cr    # Crystal (testing)
```

### Codegen Debugging with mzd
```bash
# 1. Compile to .a80 (has ABI comments) and binary
mz program.minz -o program.a80
mza program.a80 -o program.bin

# 2. Disassemble with register annotations
mzd --regs program.bin               # shows IN/OUT/CLOBBER per function

# 3. Verify codegen against compiler-declared ABI
mzd --regs --verify-abi program.a80 program.bin
# Output: "ABI verify: 5/5 functions matched, all OK"
# ...or mismatches like:
#   board_set  IN: extra=D (declared=A,C,B detected=A,C,B,D)  ← codegen bug!

# 4. Other useful mzd flags
mzd --cycles program.bin              # T-state counts per instruction
mzd --regs --stats program.com        # full analysis with statistics
mzd -t cpm --regs program.com         # CP/M platform (auto-detect BDOS ABI)
```

**How --verify-abi works:**
- Parses `; fun name(p: type = REG) -> type = REG ; clobbers: REG` from .a80
- Assembles .a80 internally to resolve label→address mapping
- Compares declared IN/OUT/CLOBBER vs detected (provenance-tracked) register usage
- Mismatches = likely codegen bugs (register allocator, calling convention, clobber)

**Provenance tracking** traces values through `EX DE,HL`, `LD r,r'`, and PUSH/POP chains. ABI-aware CALL consumption uses BDOS/ROM profiles to avoid false inputs.

## 📁 Project Structure

```
minz/
├── minzc/              # Go compiler
│   ├── cmd/           # CLI tools (minzc, mza, mze, mzd, mzx, mzrun, ...)
│   ├── pkg/           # Compiler packages
│   │   ├── spectrum/  # MZX ZX Spectrum emulator
│   │   ├── emulator/  # Z80 CPU emulator (FUSE-tested)
│   │   ├── z80asm/    # MZA assembler
│   │   ├── disasm/    # MZD disassembler
│   │   └── ...        # Parser, semantic, IR, codegen, etc.
│   └── tests/         # Test files
├── stdlib/            # Standard library
│   ├── math/         # fast.minz, random.minz
│   ├── graphics/     # screen.minz
│   ├── input/        # keyboard.minz
│   ├── text/         # string.minz, format.minz
│   ├── sound/        # beep.minz
│   ├── time/         # delay.minz
│   ├── mem/          # copy.minz
│   ├── cpm/          # bdos.minz (CP/M BDOS API)
│   └── agon/         # mos.minz, vdp.minz (Agon Light 2)
├── examples/          # MinZ programs
├── docs/             # Technical documentation (by topic)
├── reports/          # Progress reports (date-numbered)
└── releases/         # Release packages
```

## 🎯 Design Philosophy

### Ruby-Style Developer Happiness
```minz
// Flexible function declaration
fn add(a: u8, b: u8) -> u8 { ... }    // or 'fun'
fun subtract(a: u8, b: u8) -> u8 { ... }

// Clear global variables
global counter: u8 = 0;

// Function overloading
print(42);     // No more print_u8!
print("Hi");   // Just print!
```

### Target Architecture
One backend, multiple targets:
```bash
mz program.minz -b z80 --target=spectrum  # ZX Spectrum
mz program.minz -b z80 --target=cpm       # CP/M
mz program.minz -b z80 --target=agon      # Agon Light 2 (eZ80)
```

### Agon Light 2 Support (NEW)
Native eZ80 support with MOS/VDP APIs:
```minz
import stdlib.agon.mos;
import stdlib.agon.vdp;

fun main() {
    set_mode(MODE_320x240x64);
    fill_circle(160, 120, 50);
    mos_puts("Hello Agon!");
}
```

## 📊 Current Metrics (v0.23.0, verified 2026-03-24)

| Metric | Value |
|--------|-------|
| Nanz examples | **35/35** (100%) — all compile + assemble. [Report #110](reports/2026-03-24-110-Reliability-Sprint-35-of-35.md) |
| Core examples | 71/73 (97%) |
| All examples | 131/173 (75%) — failures in agon, cpm, feature_tests, zvdb, zx_demos |
| Stdlib modules | 12 documented (real), ~35-40 of 55 files compile |
| Z80 emulator coverage | 100% (1335/1335 FUSE) |
| Peephole patterns | 67 (asm) + MIR passes |
| Production backends | 1 (Z80) + 1 partial (C) + 1 QBE (correctness oracle) + 8 experimental |
| LIR backend | **948/948 pipeline** (100%), **97.9% VM-verified** (C89/risc32) — PBQP→WFC guided, IXH/IXL spill, __mul8/__mul16, tail call opt |
| VIR backend | **645/645 functions** (100%), **55/55 Z80-verified**, **-60% vs SDCC** — Z3 unified solver, Z3-PFCCO, inline runtime, zero PBQP fallback |
| MIR backend tests | 9/11 pass, 2 known bugs (ADR-0006) |
| Frontends | 8 (Nanz, C89, PL/M, Lanz, Lizp, Pascal, ABAP, **Frill**) — all route through HIR→MIR2→Z80 |
| Frill (.frl) | ML-style functional: 38 features — let, if/then/else, pipe \|>, compose >>, match+guards, ADT, lambda, currying, tuples, type classes, QTT linearity (!/~), while, for, mutation, peek/poke, property testing. 1000+ compile-time checks. See [Frill Guide](docs/Frill_Language_Guide.md) |

> **Note on C frontend:** C17 conformant (freestanding) + C23 extensions. First Z80 compiler
> with C11/C17. Use for benchmarking against SDCC. Recommend **Nanz** as primary language.
>
> **C test asserts:** Always add DUAL asserts for new C test programs:
> ```c
> // assert func(args) == expected via mir2
> // assert func(args) == expected via z80
> ```
> `via mir2` = MIR2 VM (fast, u16 arithmetic). `via z80` = full Z80 compile+emulate.
> Exceptions: pointer tests → z80 only. u8 overflow → z80 only (mir2 does u16).
> Test files: `examples/c89/` (legacy C89) and `examples/c/` (C99+, new tests go here).
| C89 corpus | 350/350 asserts, 38 files in `examples/c89/` (MIR2) |
| C99+ corpus | 174/174 asserts, 13 files in `examples/c/` (MIR2) — C99/C11/C23 |
| C total | **524/524** asserts across 51 test files |
| C conformance | **C17** (freestanding) + C23 extensions (#embed, nullptr, bool) |
| ABAP examples | 8 programs (hello, fibonacci, fizzbuzz, guessing, bubblesort, forms, oop, sysinfo) |
| E2E Z80 tests | 24 (fibonacci, flag-return, div8, div16, mod8, divmod-combined + 6502) |
| Parser | Participle (native Go, zero deps) |
| Toolchain binaries | 9 working (mz, mza, mze, mzx, mzd, mzlsp, mzrun, mztap, mzv) + mzv1 (MIR1) + mzr (broken) |
| Go test packages | 26/26 pass, 0 fail |

---

## 🔧 Toolchain Component Status

| Tool | Status | Description |
|------|--------|-------------|
| **MZC** | ✅ DONE | MinZ Compiler (Go) |
| **MZA** | ✅ DONE | Z80 Assembler (table-driven encoder) |
| **MZE** | ✅ DONE | Z80 Emulator (1335/1335 FUSE tests) |
| **MZX** | ✅ DONE | ZX Spectrum emulator (T-state accurate, Ebitengine) |
| **MZD** | ✅ DONE | Z80 Disassembler (IDA-like analysis, `--regs` IN/OUT/CLOBBER, `--verify-abi`) |
| **MZLSP** | ✅ DONE | Language Server Protocol (diagnostics, hover, goto-def, completion) |
| **MZRUN** | ✅ DONE | Remote runner (DZRP) |
| **MZTAP** | ✅ DONE | TAP file loader |
| **MZV** | ✅ DONE | MIR2 VM runner with TUI display + ZX font OCR (Tetris verified) |
| **MZV1** | ⏸️ ARCHIVED | MIR1 Virtual Machine runner (superseded by MZV) |
| **MZR** | ❌ BROKEN | Interactive REPL (compileModule returns empty, :run unimplemented) |
| **DAP** | 📋 TOBE | Debug Adapter Protocol (native, beyond DeZog) |

### MZRUN Usage
```bash
# Start ZXSpeculator with DZRP on port 11000, then:
./mzrun program.minz --reset -v
```

## Documentation System

### Two directories, two purposes:

| Directory | Purpose | Naming Convention | Examples |
|-----------|---------|-------------------|----------|
| `reports/` | Progress reports, analysis, status updates | `YYYY-MM-DD-NNN-Topic.md` | `2026-02-23-009-MZX_Phase2_Progress_Report.md` |
| `docs/` | Technical documentation, guides, references | `Topic_Name.md` (or date-prefixed for legacy) | `INTERNAL_ARCHITECTURE.md`, `Metafunction_Design_Decisions.md` |

### Reports (`reports/`)
Date-numbered, chronologically sorted. Tracking progress over time.
- Format: `YYYY-MM-DD-NNN-Topic.md`
- Sequential NNN counter (check `ls reports/ | sort | tail -1` for next number)
- Content: progress reports, analysis results, benchmarks, status snapshots

### Docs (`docs/`)
Topic-organized, canonical reference material. One file per topic.
- Format: `Topic_Name.md` (descriptive, underscored)
- Legacy files keep their date-prefixed names
- Content: architecture guides, design decisions, ADRs, API references

### Workflow for Claude
```
# Writing a progress report:
Write to: reports/YYYY-MM-DD-NNN-Topic.md

# Writing technical documentation:
Write to: docs/Topic_Name.md

# DO NOT use inbox/ — write directly to the correct directory
```

### Finding Documents
```bash
ls reports/ | sort          # Reports chronologically
ls docs/ | sort             # Docs alphabetically
grep -rl "TSMC" docs/      # Find by topic
ls reports/2026-02-*        # All February 2026 reports
```

## 🤖 AI Colleague Consultation

**Purpose**: Leverage AI tools (GPT-4, o4-mini, Claude) as virtual colleagues for architectural decisions, debugging, and design reviews.

### When to Consult
- Major architectural choices (parser strategy, optimization approaches)
- Stuck issues or nonobvious bugs  
- Design trade-offs and brainstorming
- Sanity-checking assumptions before large refactors

### How to Consult Effectively
1. **Provide full context**: Problem statement, what you've tried, relevant code snippets, constraints
2. **End with specific ask**: "Pros/cons of ANTLR vs hand-written parser" vs "Help with parser"
3. **Cross-check multiple models**: Run same question through 2+ AI colleagues for consensus
4. **Include concrete constraints**: Performance targets, maintenance concerns, team skills

### Evaluating AI Advice
- Treat as input for team discussion, not final authority
- Cross-check factual claims against official docs
- Plan small proof-of-concept to validate suggestions  
- When multiple models agree, confidence increases (but still validate)

### Documentation
Keep an **AI Consultation Log** in relevant docs:
- Date, participants (which AI models), original prompt
- Key advice given and follow-up questions
- Outcome and rationale for following/rejecting advice
- Link to related issues/PRs/commits

**Example Success**: August 2024 - Consulted GPT-4 and o4-mini on ANTLR vs hand-written parser for Z80 assembler. Both recommended keeping hand-written parser and fixing encoder issues instead. Decision saved significant development time and led to identifying the real problem.

### Best Practices
- Never merge critical changes solely on AI advice
- Always do code reviews and team discussion for broad-impact decisions
- Create prompt templates for consistent, high-quality consultations
- Review consultation logs in retrospectives to improve question quality

## 🔧 Documentation Style: "Pragmatic Humble Solid"

- ✅ **Transparent**: "Core features work" / "Experimental"  
- 🚧 **Status indicators**: Working/In Progress/Missing
- 📊 **Specific**: "60% of examples compile"
- ⚠️ **Honest warnings**: "Not production ready"

Celebrate real achievements without hype. Ground excitement in facts.

---

*MinZ: Modern programming abstractions with zero-cost performance on vintage Z80 hardware.*