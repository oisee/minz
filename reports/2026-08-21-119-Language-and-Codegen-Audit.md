# Language and Codegen Audit: Frontends, TSMC, Spill Tiers, Self-Hosting

**Date:** 2026-08-21
**Method:** survey by a dedicated agent that verified by *executing* — building `mz`/`mza`/`mze`,
compiling all eight corpora, and hand-assembling functions with ABI-correct bootstraps to run on
the Z80 emulator rather than trusting the repo's own assert harness.
**Companions:** [116](2026-08-21-116-Prior-Art-and-Novelty-Audit.md) (prior art),
[117](2026-08-21-117-Solver-Architecture-Audit.md) (solvers),
[118](2026-08-21-118-GPU-Superoptimizer-Artifacts-Audit.md) (GPU artifacts).

---

## Headline: a one-line change is depressing every metric in the repo

**The default backend miscompiles ordinary loops.** `--vir` defaults to `true`
(`cmd/minzc/main.go:223`). Verified on the emulator across four independent loop shapes:

| Program | VIR (default) | `--lir` | `--vir=false` (PBQP) | Correct |
|---|---|---|---|---|
| `while i != 0 { c=c+1; i=i-1 }`, n=5 | **0** | — | 5 | 5 |
| `while i <= n { s=s+i; i=i+1 }`, n=5 | **1** | — | 15 | 15 |
| `for i in 0..n { s = s + i }`, n=5 | **0** | **0** | 10 | 10 |
| `range(0..n).fold(0, \|a,i\| a+i)`, n=5 | **5** | — | 15 | 15 |

This corroborates Report 117's `gcd` finding from four more directions, and PBQP is correct on all
of them.

**The project already made this decision and never implemented it.**
`reports/2026-04-08-Session-Report-EN.md` records two agents independently concluding that "Z3
breaks on any while/for with a parameter-bound loop counter", that Z3 is "a cannon aimed at
sparrows", and that the resolution was **"Z3 parked as `--vir` flag, PBQP stays production
default."** `pkg/pipeline/pipeline.go:131` (`DefaultOptions`) correctly leaves VIR off. The CLI
flag at `main.go:223` turns it back on for every user.

The blast radius is wide: ABAP goes from **0/30 assembling under VIR to 28/30 under `--lir`**;
Pascal from 9 to 11 compiling.

---

## 1. The frontend landscape

All eight genuinely converge on `*hir.Module` through one switch (`main.go:748-861`). That part of
the claim holds. Compile rates at default flags, 45s timeout:

| Frontend | LOC | Compiles | Assembles | Verdict |
|---|---|---|---|---|
| **Nanz** | 13,725 (172 test funcs) | 55/57 | — | Real primary language, most-tested package |
| **C89/C99/ObjC** | 27,109 + 9,292 | 36/39, 19/20, 11/11 | 33, 18, 8 | Most capable — but `pkg/cparse` is a **vendored fork of `modernc.org/cc/v4`** (upstream still in `go.mod:41`). Our work is the ~4,300-LOC lowerer + ObjC extensions |
| **ABAP** | 4,840 + a **14 MB embedded `abap_parser.wasm`** | 30/30 | 0 (VIR) / 28 (LIR) | No parser of its own — shells to abaplint via wazero. **8 statement forms**; `LoopStmt`/`CallFunctionStmt`/`CreateObjectStmt` exist in the AST and are never constructed. 3 tests for 4,693 LOC |
| **Pascal** | 3,772 (31 tests) | 9/11 (11/11 on LIR) | 7 | Genuine hand-written TP3.0 subset. No units, no sets; multi-dim arrays silently 1-D |
| **PL/M** | 3,723 (23 tests) | 3/5 | 1 | Real parser + `LITERALLY` preprocessor, but **silent** drops: `STRUCTURE`→`BYTE`, `INITIAL` discarded, `GOTO` lowered to a fabricated intrinsic that cannot work |
| **Lizp** | 1,241 (24 tests) | 6/14 | 3 | `and`/`or` are **bitwise, not short-circuiting**; unknown types default to `u8`, so a typo compiles clean at the wrong width |
| **Lanz** | 3,064 (41 tests) | — | — | Not a user language: an S-expr surface over HIR. **4 `.lanz` files exist in the whole repo** |
| **Frill** | 2,715 (14 tests) | 17/18 | 13/17 | See below |

Two documentation corrections:

- **`.minz` is not a second frontend.** `main.go:754` is `case ".nanz", ".minz":` — both go to the
  Nanz parser. Several docs still describe a "frozen MIR1 frontend" that no longer exists.
- **`pkg/objc` (1,158 LOC) is orphaned** — zero importers; `.m` routes to `pkg/c89/objc_lower.go`.
  Same for `pkg/hirwat`.

### Frill is an advanced demo, not a language

One 2,436-LOC file, **no type checker and no lowering pass** — the parser emits `hir.Func`
directly and unknown types default to `TyU8`. The QTT linearity claim does not survive contact:
only two of three sigils exist (no `&`/ω, so no semiring), quantities attach only to *function
parameters*, the checker is a syntactic occurrence count that gets branches wrong
(`if c > 0 then x else x` is rejected as "used 2 times"), and it is **unsound** — `countUsesStmt`
walks only `ReturnStmt`/`VarDeclStmt`, so an *erased* (`~`) parameter can be read every iteration
inside a `while` and compile clean. Zero tests, zero examples, zero stdlib files use it. Type
classes are name-mangling with no dispatch (`show 5` → "function @show not found"). "1000+
compile-time checks" is `12 properties × 256` = 87.5% of the figure.

What *is* good in Frill: match exhaustiveness checking, an IO-effect check that really rejects
impure calls, and monomorphized parametric polymorphism.

---

## 2. TSMC: the interesting version is unbuilt

Three disconnected implementations:

- **Legacy "TRUE SMC"** (`pkg/optimizer/true_smc.go`) is complete and *works* — reachable only by
  renaming a file to an unknown extension. It emits parameter anchors, a `PATCH_TABLE`,
  caller-side patch stores, assembles to 94 bytes, and the emulator write-heatmap proves genuine
  self-modification. **It is unreachable**: `main.go:258` routes every real extension to
  `compileViaHIR`. Alongside it, `pkg/optimizer/tsmc_patterns.go` (175 LOC) is registered in the
  pass list and is a pure no-op — every function only `fmt.Printf`s.
- **`@smc` parameters** are the one live SMC feature, and run correctly on the emulator under
  `--lir`. Three real defects: it **doesn't assemble under the default VIR backend**;
  `ConstantCallElim` has no guard for `OpPatchSlot`, so a function reading an SMC slot is treated
  as pure and folded to the slot's *initial* value — silent wrong code; and there is no recursion
  guard on `@smc` at all.
- **`OpPatch` on Z80 is a comment.** `mir2/z80codegen.go:3743` emits
  `; TODO: LD (patch_addr_%s), %s`. `TestZ80CodegenSMCCounter` **passes** while printing that TODO.

### TSMC tunnels are paper-only

`grep -rn "tunnel" --include="*.go" minzc/` returns **nothing**. The word exists only in
`CLAUDE.md` and in ADR-0038/0039, both `Status: Proposed`. `docs/145_TSMC_Complete_Philosophy.md`,
cited twice in `CLAUDE.md`, **does not exist** (it is under `docs/_archive_2025/`). Across 314
generated `.a80` files, **zero** `_tsmc_` sites.

The nearest real code, `pkg/mir2/tsmc_spill.go` (227 LOC), implements exactly the ADR's idea and
**cannot fire**: it locates reload sites by string-matching `$%04X`, but `g.loc()` for `LocMem` was
changed to return a named `_spill_*` label, so all nine call sites return `nil` — and there is a
band-aid, `fixOrphanedTSMCStores`, that rewrites the orphaned patch-stores back into ordinary
spills.

The repo's own TSMC benchmark is the sharpest illustration.
`pkg/z80testing/tsmc_benchmark_report.txt`: **"Total Benchmarks: 10, Passed: 0 (0.0%)"** — all ten
fail to compile with stale syntax. The same file then states "Recursive algorithms … 35-40%
improvement", which is a **hardcoded string literal** at `tsmc_benchmarks.go:858`, not a
measurement.

---

## 3. Zero-cost iterators: the fusion is real, the DJNZ mostly isn't

Fusion genuinely works (`hir/lower.go:2431`). `03_filter_map_chain.nanz` compiles to one loop with
both lambdas inlined and deleted (`; pruned functions: 2`). That is real deforestation.

But **there is exactly one `TermDJNZ` construction site in the entire compiler**
(`mir2/builder.go:444`), reachable from exactly one place (`hir/lower.go:3045`, `lowerRangeLoop`):

- `range(lo..hi).forEach(...)` → hand-written quality: `LD A,D / OR A / JR Z,exit / .body: CALL
  sink / DJNZ .body`.
- Array/pointer `forEach` → correct but **no DJNZ**. Measured back-edge: `LD A,1 / NEG / ADD A,B /
  LD B,A / JRS` plus a head test, ≈48T of loop overhead where `DJNZ` is 13T. An older cached
  `.a80` of the same file *does* contain `DJNZ` — this is a **regression**, and it affects
  `--vir=false` too.
- `map`/`filter` stages give **wrong answers on Z80** while passing on the MIR2 VM
  (`.map(|x| x*2).forEach` over `[1..5]` → 20, expected 30).

The advertised evidence is stale rather than wrong-at-the-time:
`docs/Iterator_Implementation_Status.md` is dated 2026-03-02 and its 11/11 hex-verified table was
measured on the old `.minz` frontend. Re-running `examples/e2e_iterators/run_e2e.sh` today gives
**0 passed, 11 failed** — the fixtures use syntax the Nanz parser no longer accepts.

Prior art: deforestation/stream fusion is 1990s Haskell (Wadler; Coutts et al.), standard in Rust
and Java. Nothing novel; the range-DJNZ output is simply good Z80 codegen.

---

## 4. Interfaces, CTIE, metafunctions

**Interfaces** are real but shallow — and the design decision is the interesting part.
`parseInterfaceDecl` records **method names only**; signatures are skipped tokens, so there is no
conformance checking. Dispatch resolves a unique implementor to a direct `Type_method` call and
**refuses to compile** 0 or ≥2 implementors. No vtable exists by construction — confirmed in asm:
direct `JP feed`, no `JP (HL)`, no jump table. *Refusing ambiguous dispatch rather than falling
back to a vtable is an honest, consistently-implemented language decision* and deserves to be
stated as such.

**CTIE is dead code that miscompiles.** `pkg/ctie` is imported by exactly one file on the
unreachable path; `--disable-ctie` is a provable no-op. Forced to run, `add(10, 20)` yields
`LD HL, 0` because `const_tracker.go:139` fabricates arguments (`// Simplified: assume args in
r1, r2`). The advertised behaviour is delivered by an unrelated, correct pass —
**`pkg/mir2/consteval.go ConstantCallElim`**, which runs the callee on the MIR2 VM with a step
budget. Verified: `return fib(10)` → `main: LD DE, 55 / RET`.

**Metafunctions: `CLAUDE.md` documents a superseded design.** `@minz[[[...]]]`, `@define`, `@lua`,
`@emit`, `@if/@elif/@else` are **all paper-only** — their AST nodes are declared and never
constructed; the 963-LOC `semantic/minz_interpreter.go` is unreachable; gopher-lua is imported once
and never reached. `@define` is rejected on both paths. `CLAUDE.md`'s "@define — FULLY IMPLEMENTED
AND WORKING!" is refuted, and `docs/Metafunction_Design_Decisions.md`, which it cites, does not
exist.

**What replaced them is better and undocumented.** ADR-0023 (Accepted) specifies metafunctions as
*Nanz functions executed at compile time on the MIR2 VM*, using Lanz as the interchange format. It
works: `examples/nanz/meta_screen.nanz:35` defines `fun @screen(title: ^u8) -> void` which walks
the block's AST as a typed struct array and calls `emit(...)`. Same language for compile-time and
runtime, typed AST introspection, no external interpreter. `CLAUDE.md` doesn't mention it.

**`@error`/`@check`/`@propagate` do not work end-to-end** — they emit calls to undefined labels;
the label audit detects it and `mz` still exits 0. ADR-0016's replacement syntax is marked
**Accepted** and does not parse. The idiom that actually works is ADT
`enum Result { Ok(u8), Err(u8) }`.

**Function overloading does not exist in the shipping frontend.** `resolveCall`
(`nanz/parse.go:5099`) is a bare map lookup — no mangling, no ambiguity check; duplicates
overwrite each other silently. The real implementation (`semantic/mangling.go`,
`overload_resolution.go`) is on the dead path. The codebase compensates by hand-mangling
(`print_u8`, `min8`/`min16`) — manually doing what overloading was meant to hide.

**Operator overloading** dispatch is real, but `widemath.nanz`'s celebrated **31 asserts are all
`via mir2`** — none run on Z80, and `--asserts=z80` passes vacuously because `RunAssertsZ80` skips
`Via=="mir2"`. Forced onto Z80: under VIR `op_mul_u8_u8` never terminates (killed at 11m21s);
under `--vir=false` it terminates wrong (`area(10,20)` → 40, want 200).

There is also a genuine language hazard worth naming: overloading operators on *primitives*,
module-wide, with no escape hatch. Once `fun *(a: u8, b: u8) -> u16` exists in a file, every
`u8 * u8` in that file is that call — **including inside the overload's own body**. C++, Rust and
D all forbid this for exactly that reason.

---

## 5. Self-hosting is ~5% real

Matching the project's own internal estimate (`contexts/2026-03-29-session-15-wisdom.md:34`).

- **982 unique Nanz LOC vs 240,557 non-test Go LOC = 0.41%.** `self_tokenizer.nanz` (542) is a dead
  duplicate. The claimed "480 LOC" matches nothing at any commit.
- **It has never executed as Z80.** Stages 1 and 2 run on **MZV**, which is a Go program that
  compiles the `.nanz` with the Go frontend and then interprets MIR2. Compiled to Z80 the binary is
  dead on arrival: every `@extern` host function lowers to an empty label that falls through.
- **Stage 2 is not connected to Stage 1** — the S-expression is a hardcoded string literal.
- **Stages 3-5 are not "Go helpers" — they are the entire Go compiler.** The one artifact labelled
  a Stage 3/4 helper, `cmd/mir2asm`, reads a bespoke format **nothing in the repo emits** (zero
  `.mir2` files).
- The headline demo reproduces for exactly one function. Beyond that: **≥2 functions produces no
  output file at all** (arenas at 0x8000/0x9000/0xB000 overwrite the VM's string constants); the
  second function receives the first function's body; and `parse_expr` runs three sequential
  `while` loops (all `+`, then all `-`, then all `*`), so `a + b * b` silently parses as
  `(a+b)*b` and emits working-but-wrong Z80.
- **No test, no script, no CI** references `self_parser`/`selfhost`/`mir2asm`.

Fair label: *a promising two-stage front-end demo, currently regressed, carrying the marketing
label of a bootstrap.* `self_parser.nanz` cannot parse `self_parser.nanz`.

---

## 6. The spill-tier hierarchy is one real tier plus a wish-list

| Tier | Declared in descriptor | Allocator can place a value there? | In real output? |
|---|---|---|---|
| L0 A–L | yes (all 3) | yes | everywhere |
| **L1 IXH/IXL/IYH/IYL** | yes | **yes** | **yes — 32-35/142 files** |
| L2 `I` register | **no** | no | 0 |
| L2b `R` register | **no** | no | 0 |
| L3 TSMC slot | yes, cost 20 | **no** | 0/314 files |
| L3b Shadow / EXX | yes, cost 8 | **no** | `EXX` never emitted |
| L4 PUSH/POP | yes | **no** (`InfCost`) | ad-hoc save-across-call pass only |
| L5 memory spill | yes | yes | 11/142 files |

`pkg/vir/z80.go:107-120` declares the full cost table (1/8/20/26/22) and then `z80.go:150` removes
almost all of it: `NonAllocatable = … .Or(Z80_Shadow8).Or(Z80_TSMC).Or(Z80_Mem).Or(Z80_Stack)`,
commented *"EXX zone support is not yet wired… let PBQP handle spilling instead."*
`pkg/mir2/z80cost.go` is blunter: `LocShadow → InfCost`, `LocStack → InfCost`. There is no
`LocTSMC` in `pkg/mir2` at all, and no `LocIReg`/`LocRReg` anywhere. LIR's `tsmc0..7` are emitted
as literal data bytes (`tsmc0: DB 0`). ADR-0038 admits this: *"Assembly output already has
`_tsmc0..7` slots (unused, emitted by LIR)."* The 18T figures for I/R appear nowhere in code.

**What is real and does matter: IXH/IXL/IYH/IYL as first-class allocatable 8-bit registers with a
T-state cost model and DD/FD prefix legality rules.** Confirmed in fresh output (13 uses in
`08_arena_allocator`, all legal encodings); a pressure test with 11 live values spilled to **zero**
memory slots because the four index halves absorbed the overflow. `BUG-008`'s impossible
`LD IXL, (IX+d)` no longer reproduces.

A second, subtler real technique: **`A'` via `EX AF,AF'` as a scratch bracket** for spill-to-spill
and prefix-conflicting moves (`mir2/z80codegen.go:1186-1223`) — present in 10/142 files. Not what
the docs call L3b, but real.

**Recursion safety is inert.** `isRecursive` detects direct self-calls only, and its sole consumer
is the dead TSMC scan. `Func.Attrs.IsRecursive` is documented as "computed (directly or
transitively)" but the only assignment in the whole pipeline is
`grace_runner.go:1134: f.Attrs.IsRecursive = false` — so the guards in `consteval.go:96` and
`inline.go:89` never fire.

---

## 7. Where the defensible novelty is

Consistent with Report 116, from a different angle. Ranked by confidence:

1. **PFCCO — per-function calling conventions for irregular register files.** Implemented
   (`pkg/mir2/contracts.go`: greedy DP on the topo-sorted call graph, 4-pass convergence), written
   up, and correctly positioned against Krause 2013/2022 and the ARM link-time work. Measured on
   the repo's 8-program microbenchmark: **-29% aggregate**, best single case -66%. Note the harness
   has SDCC's counts **hardcoded and hand-counted**, compares static instruction counts, and ran
   SDCC with no optimization flags. `CLAUDE.md`'s "-60%" and the CLI's "-71%" are not supported by
   anything runnable; **-29% on eight tiny functions is.**
2. **Retrieval-based codegen — the ~80-88% signature-reuse result.** The claim that backend
   constraint spaces on small irregular ISAs have low effective entropy, so allocation can move
   from online search to offline knowledge compilation. See Report 118 for the artifact reality.
3. **IXH/IXL/IYH/IYL as costed allocation targets.** Modest but real. State it as "we are not aware
   of" rather than "no compiler does" — see Report 116 §6.
4. **Dual-backend differential asserts as a language feature.** `assert f(x) == y` in source,
   executed at compile time by *both* the MIR2 VM and a real Z80 emulator, with parameters placed
   using the actual register allocation. 1,366 asserts across the corpora. **This is the strongest
   engineering idea in the repo and it is under-sold** — and currently half-broken:
   `buildAssertBootstrap` reads the PBQP `AllocResult` rather than the VIR PFCCO ABI, producing
   false failures, and ~96% of C corpus asserts are `via mir2`, so "524/524 C asserts" is a
   MIR2-VM claim, not a Z80 claim.
5. **Range-refined integer types driving automatic tabulation.** `fun popcount(x: u8<0..255>)` →
   LUTGen evaluates all 256 inputs on the MIR2 VM and replaces the body with a table lookup
   (verified). ADR-0015 cites Ada subtypes/SPARK/Zig honestly. Turns a manual retro technique into
   a type-system feature. (Minor gap: no `ALIGN 256`, so it costs `ADD HL,BC` instead of `LD L,A`.)
6. **Compile-time metafunctions in the target language itself** (ADR-0023) — strictly better than
   the `@define`/`@lua` design `CLAUDE.md` still documents.

### The integration thesis, assessed

ADR-0034 argues every optimization is a CSP and that ISLE + WFC + PBQP should replace hand-written
passes at every level. That is a coherent research programme. **It is not yet demonstrated** — the
three-solver stack does not currently beat the thing it replaces: PBQP, the legacy path, is correct
on every loop that VIR and LIR miscompile. The honest current claim is *"eight frontends share one
Z80 backend and a differential test harness"*, which is genuinely valuable, rather than *"three
cooperating solvers produce provably optimal code"*.

### A counter-example worth crediting

The agent swept **all 254 constant multipliers** (`x * k`, k=2..255, x=7) with entry carry both
clear and set and reported zero wrong values.

**This disagrees with Report 118 §Headline and needs reconciling.** The direct demonstration there
stands and is three commands to reproduce: assemble the emitted `a * 23` body, `SCF` before the
call → 169; `OR A` before the call → 161. `RLA` reads CY and nothing defines CY before it, so the
dependence is a property of the instruction sequence, not of the harness. The most likely
explanation is that the sweep's bootstrap did not actually vary carry *at the call boundary*.
Either way the exposure is latent rather than active — only 2 corpus sites fire the table today and
both happen to contain `ADC` — and the guard has since been widened
(`fix/vir-mul-table-carry`, commit `eaf96cd9`), taking coverage from 12 to 63 of 103 entries.

Also worth crediting: `mz` round-trips its own output through the assembler as a validation pass
(`pkg/z80validate/validate.go:27`), which caught 9 illegal `AND 511` instructions when FatFS
(7,249 lines of real C) went through it. Good discipline — though it cannot catch semantic errors:
`unsigned short mask(unsigned short x){return x & 511;}` emits `AND H / AND L` (the immediate
silently replaced by the register) and returns 1023 for 1023.

---

## Ranked directions

1. **Flip `--vir` to default false** (`main.go:223`) — one line, implementing a decision the
   project already made and documented in April. Restores loop correctness, un-breaks `@smc`
   patcher emission, fixes indirect calls (`vir/bridge.go:926` gates arg setup on `inst.Sym != ""`,
   so function pointers become `CALL 0`), and takes ABAP from 0/30 to 28/30 assembling. Everything
   else is easier to evaluate once the default is trustworthy.
2. **Fix `buildAssertBootstrap` to read the emitted ABI**, then make the audit non-advisory: fail
   the build on `LABEL AUDIT` warnings and add an assemble step to the corpus gate. Today the
   compiler's best feature reports false failures on VIR output and its best signal exits 0. The
   77-compile / 60-assemble gap is invisible in every published metric.
3. **Re-point the asserts at Z80.** The MIR2 VM is a u16 interpreter that masks exactly the
   register-allocation, calling-convention and flag bugs found across these four reports.
   Converting even 20% of the `via mir2` asserts to dual-run would have caught the `map`/`filter`,
   `gcd`, and `u8*u8` defects at authoring time.
4. **Restore DJNZ for array/pointer `forEach`** — one `TermDJNZ` site reached from one place.
   Emitting it from `lowerForEach` too recovers ~35T/element on the flagship abstraction.
5. **Write the PFCCO paper; it is the strongest asset.** It needs a stated benchmark protocol
   (metric, SDCC flags, semantic-equivalence criterion) and edge-weight-aware costs. Retire
   "-60%"/"-71%" in favour of the number the harness actually prints.
6. **Decide about the dead path and say so.** `pkg/parser` + `pkg/ast` + `pkg/semantic` + `pkg/ir` +
   `pkg/codegen` + `pkg/ctie` + `pkg/metafunction` (~50K LOC) are unreachable for every documented
   extension, along with `pkg/objc` and `pkg/hirwat`. Meanwhile `CLAUDE.md`'s marquee sections —
   TSMC tunnels, the L0-L5 hierarchy, `@define`/`@minz`/`@lua`, CTIE, function overloading —
   document that path or unbuilt ADRs, while the things that *do* work (ADR-0023 metafunctions,
   ADR-0015 LUTGen, dual-VM asserts, `clob auto` inline asm) go unmentioned. **A CLAUDE.md that
   matched `git grep` would be the highest-leverage documentation change in the repo.**
7. **Vendor or generate the GPU artifacts** — until a clean checkout reproduces the codegen, the
   retrieval-based-compilation result cannot be independently verified, which is precisely what a
   reviewer will ask for.
