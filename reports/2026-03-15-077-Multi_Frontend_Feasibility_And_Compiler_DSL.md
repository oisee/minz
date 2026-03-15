# Report #077 — Multi-Frontend Feasibility & Compiler-Internal DSL

**Date:** 2026-03-15
**Status:** Research / Proposal
**Context:** Evaluating new language frontends for MinZ and internal optimization DSL

---

## Architecture: All Roads Lead to HIR

Every frontend produces `*hir.Module`, which feeds into the shared pipeline:

```
                            ┌─── Nanz   (production, native Go parser)
                            ├─── Lanz   (S-expr, roundtrippable)
                            ├─── PLM    (PL/M-80, retro compat)
User-facing frontends ──────┼─── Lizp   (proposed: Lisp sugar over Lanz)
                            ├─── Pascal (proposed: hand-written parser)
                            ├─── C89    (proposed: modernc.org/cc + lower.go)
                            └─── Datalog (proposed: @facts → LUT)
                                    │
                              *hir.Module
                                    │
                              HIR → MIR2
                                    │
                   ┌────────────────┼────────────────┐
                   │                │                 │
              Optimization    PBQP Alloc         Z80 Codegen
                   │                │                 │
              ┌────┴────┐    ┌──────┴──────┐    ┌────┴────┐
              │ Peephole │    │  EdgeCost   │    │  emit   │
              │ 67 rules │    │  ADR-0020   │    │  .a80   │
              └─────────┘    └─────────────┘    └─────────┘
```

HIR supports: functions, globals, structs, arrays, if/while/for/switch,
pointers, casts, break/continue, multi-return, asm blocks, assertions.

Types: u8/u16/i8/i16/u24/u32/bool/ptr/arrays/structs/tuples/slices/fixed-point.

---

## Part 1: User-Facing Frontends

### Tier 1 — Ready in Days

#### Lizp (Compiled Lisp) — 95% ready, 1-2 days

Lanz IS already S-expressions. Lizp adds thin sugar:

```lisp
;; Lizp input:
(defun piece-color (t : u8) -> u8
  (cond
    ((= t 0) #x28)
    ((= t 1) #x30)
    (t       #x08)))

;; Translates to Lanz:
(fun piece_color ((t u8)) u8
  (if (== t 0) 40
    (if (== t 1) 48
      8)))

;; → hir.Module → MIR2 → Z80
```

**What to add:** `defun`, `let`, `cond`, `defstruct`, `loop`, `dotimes`.
**What NOT to add:** cons/car/cdr (no GC), dynamic typing, eval.
**Implementation:** Syntax transformer Lizp → Lanz → HIR. ~300-400 LOC.

#### Pascal — 85% ready, 2-3 days

Pascal maps almost 1:1 to HIR. PLM frontend already shows the pattern.

| Pascal | HIR mapping |
|--------|-------------|
| `integer` | `TyI16` / `TyU16` |
| `byte` | `TyU8` |
| `boolean` | `TyBool` |
| `array[1..N] of T` | `ArrayTy{T, N}` |
| `record` | `StructTy` |
| `^T` (pointer) | `TyPtr` + `DerefExpr` |
| `if/then/else` | `IfStmt` |
| `while/do` | `WhileStmt` |
| `repeat/until` | `WhileStmt` (inverted condition) |
| `for i := a to b` | `ForRangeStmt` |
| `for i := b downto a` | `ForRangeStmt` (reversed — we have the proof!) |
| `case/of` | `SwitchStmt` |
| `procedure/function` | `hir.Func` |
| `var` params (by-ref) | `TyPtr` + auto `AddrOfExpr` |
| `set of` | Bitfield ops (`& | ^`) |
| `write/writeln` | `@print` metafunction |
| `real` | ❌ no FPU (wasn't on Z80 anyway) |

**Implementation:** `pkg/pascal/parser.go` + `lower.go`, ~800-1200 LOC.
Follow PLM pattern: `Parse() → *pascal.Module → Lower() → *hir.Module`.

**Killer feature:** Compile real Turbo Pascal CP/M programs!

#### C89 (with modernc.org/cc) — 60% ready, 2-3 days (was 1-2 weeks!)

**Key discovery:** `modernc.org/cc/v4` is a complete C99 frontend in pure Go.
BSD-3-Clause license. Zero CGo dependencies. Preprocessor + parser + type
checker included. Used by `modernc.org/sqlite` (mature, production-tested).

```go
import "modernc.org/cc/v4"

ast, err := cc.Translate(cfg, includes, sysIncludes, sources)
// ast = fully type-checked C99 AST
// We only need: cc.AST → *hir.Module (~500-800 LOC)
```

| Task | Without modernc.org/cc | With modernc.org/cc |
|------|----------------------|-------------------|
| Preprocessor | 2-3 days | ✅ free |
| Parser | 3-5 days | ✅ free |
| Type resolution | 2-3 days | ✅ free |
| Implicit conversions | 2-3 days | ✅ already resolved in AST |
| `cc.AST → hir.Module` | — | 2-3 days |
| **Total** | **1-2 weeks** | **2-3 days** |

**C99/C11/C23 features that come cheap:**

| Feature | Cost | Notes |
|---------|------|-------|
| `//` comments | Free | modernc.org/cc handles it |
| Mixed declarations | Free | HIR already works this way |
| `_Bool` / `bool` | Free | `TyBool` |
| `_Static_assert` | Free | `hir.Assert` |
| `inline` | Free | Ignore (compiler decides) |
| Designated initializers | Free | cc resolves to ordered |
| Digit separators | Free | Lexer |

**Not supported (and fine for Z80):** `float`/`double`, `_Complex`, VLA,
`_Thread_local`, wide strings.

**Partial:** `union` (needs HIR extension — overlapping struct offsets),
`goto` (no HIR support — reject or transform to structured control flow),
function pointers (no HIR support — reject).

### Tier 2 — Interesting But Lower Priority

#### Datalog (compile-time facts) — 35% ready, 2-3 days

See Part 2 below for full analysis.

#### ObjC subset — 55% ready, +1-2 days on top of C89

"Objective-C without the Objective" — C89 + message syntax sugar.
`[obj message:arg]` → `obj.message(arg)` → UFCS call.
No runtime, no ARC, no NSObject. Only meaningful if C89 is done first.

#### MicroZig — 40% ready, 2-3 weeks

Zig's type system (comptime, error unions, optionals) needs significant
work. Interesting but not priority.

### Tier 3 — Won't Implement (collapse or impossible)

| Language | Why not |
|----------|---------|
| Crystal | = Nanz (same design space) |
| Ruby | = Nanz (Nanz already has Ruby interpolation, iterators) |
| Swift | = Nanz (zero-cost abstractions, value types) |
| Rust | = Nanz (ownership without borrow checker) |
| Kotlin | = Nanz |
| MicroPython | Needs GC + runtime + dynamic typing |
| JavaScript | Needs GC + prototype chain |
| Lua | Ironic (we have @lua for compile-time!), but runtime = no |
| Gleam | Needs ADTs + pattern matching + HKT — too much HIR work |

---

## Part 2: Datalog as Compile-Time Data Description

### The Insight

Datalog = facts + rules without side effects. This is exactly what
LUTGen does — but without a nice syntax.

```prolog
% Datalog: declarative piece geometry
piece_dx(i, rot0, cell0, 0).
piece_dx(i, rot0, cell1, 1).
piece_dx(i, rot0, cell2, 2).
piece_dx(i, rot0, cell3, 3).
piece_dx(i, rot1, cell0, 0).
piece_dx(i, rot1, cell1, 0).
...
```

This compiles to exactly `global PIECE_DX: [u8; 112] = [0,1,2,3, 0,0,0,0, ...]`.

### Proposed Syntax: `@facts`

Instead of a separate Datalog frontend, integrate as a Nanz annotation:

```nanz
// Current (imperative):
global PIECE_DX: [u8; 112] = [0,1,2,3, 0,0,0,0, ...]

// Proposed (@facts — declarative):
@facts
fun piece_dx(piece: u8, rot: u8, cell: u8) -> u8 {
    (0, 0, 0) => 0,  (0, 0, 1) => 1,  (0, 0, 2) => 2,  (0, 0, 3) => 3,
    (0, 1, 0) => 0,  (0, 1, 1) => 0,  (0, 1, 2) => 0,  (0, 1, 3) => 0,
    // ...
}
// Compiler generates: indexed array + lookup function
// piece * 16 + rot * 4 + cell → PIECE_DX[index]
```

The compiler:
1. Sees `@facts` annotation
2. Analyzes argument ranges (piece: 0..6, rot: 0..3, cell: 0..3)
3. Computes total table size: 7 × 4 × 4 = 112
4. Packs into 1D array with stride calculation
5. Emits LUT global + accessor function

### Derived Facts (Datalog Rules)

Rules compute new tables from existing ones at compile-time:

```nanz
@facts
fun piece_dx(piece: u8, rot: u8, cell: u8) -> u8 { ... }

@facts
fun piece_dy(piece: u8, rot: u8, cell: u8) -> u8 { ... }

// Derived: absolute board position for each cell
@derive
fun cell_board_x(piece: u8, rot: u8, cell: u8, px: u8) -> u8 {
    = px + piece_dx(piece, rot, cell)
}
// → Compiler evaluates at compile-time for all input combinations
// → Generates pre-computed table if small enough, or inline expression if not
```

### Collision Rules (the killer app)

```nanz
@derive
fun can_place(piece: u8, rot: u8, px: u8, py: u8) -> bool {
    = for_all cell in 0..4 {
        let bx = px + piece_dx(piece, rot, cell)
        let by = py + piece_dy(piece, rot, cell)
        bx < 10 && by < 20
    }
}
// Board-independent part can be pre-computed:
// → 7 × 4 × 10 × 20 = 5600 bytes (too big for Z80)
// → Compiler decides: too big, emit code instead of table
// → Falls back to runtime loop (what tetris_v2 already does)
```

The compiler can **choose** between table and code based on size constraints.
This is exactly what a smart optimizer should do — and Datalog makes the
decision space explicit.

### Implementation: Two Levels

**Level 1 — `@facts` tables (2-3 days):**
- Parse `@facts` annotation + table syntax
- Compute index strides from argument ranges
- Emit global array + accessor function
- Equivalent to what we did manually for tetris_v2 LUTs

**Level 2 — `@derive` rules with semi-naive evaluation (1-2 weeks):**
- Parse `@derive` with expressions over `@facts`
- Evaluate at compile-time (interpret the expression for all input combos)
- Decide: table vs code based on size threshold
- Full Datalog stratification for recursive rules

**Recommendation:** Ship Level 1 first. It's immediately useful for any game
data (sprite tables, color palettes, tile maps, level layouts).

---

## Part 3: Compiler-Internal Query/Pattern Language

### The Problem

MinZ currently has 67 peephole rules as hand-coded Go functions in
`z80peephole.go`. Adding a new rule means:
1. Write Go code
2. Parse asm text with string matching
3. Handle edge cases manually
4. Rebuild compiler

This doesn't scale. The register allocator (PBQP), instruction selection,
and optimization passes all have patterns that are hard-coded.

### What We Want

A declarative way to express:
- "Match this pattern in the instruction stream, replace with that"
- "Find functions that should be inlined"
- "Find cross-bank calls that need trampolines"
- "Find instruction sequences that can be fused"

### Existing Approaches in Real Compilers

| Compiler | Pattern Language | Used For |
|----------|-----------------|----------|
| GCC | `match.pd` | Tree pattern matching for GIMPLE |
| LLVM | TableGen `.td` | Instruction selection patterns |
| Graal | Ideal graph patterns (Java) | Peephole rules |
| Soufflé | Datalog | Program analysis (points-to, etc.) |
| Cranelift | ISLE (DSL) | Instruction lowering rules |

### Option A: Peephole DSL (simplest, highest ROI)

A mini-language for Z80 peephole patterns:

```
// Current Go code (z80peephole.go):
func (p *Peephole) foldLdAPair() bool {
    if !p.match("LD A, %r1") { return false }
    if !p.match("LD %r2, A") { return false }
    if p.r1 == p.r2 { return p.delete(1) }
    return false
}

// Proposed DSL:
rule fold_ld_a_pair {
    match: LD A, $r ; LD $r, A
    replace: LD A, $r
    when: true
}

rule fold_push_pop {
    match: PUSH $rr ; POP $rr
    replace: (nothing)
    when: no_side_effects_between
}

rule strength_reduce_mul2 {
    match: ADD A, A
    cost: 4T
    replaces: LD B, A ; SLA A  // was 12T
}

rule djnz_pattern {
    match:
        DEC B
        JR NZ, $label
    replace:
        DJNZ $label
    save: 1 byte, 1T
}
```

**Implementation:**
- Parse DSL at compiler build time (or embed as Go string literals)
- Generate pattern matcher from rules
- Each rule = (match template, replacement template, condition, cost)
- ~500 LOC for the DSL engine

**ROI:** Huge. The 67 existing rules become ~67 lines of DSL.
New rules added without recompiling. Testing is declarative.
Currently: ~2000 LOC of Go → ~200 lines of DSL + 300 LOC engine.

### Option B: Graph Query Language (for optimization passes)

For higher-level optimization patterns over MIR2 (not asm text):

```
// Find inline candidates:
query inline_candidates {
    match (caller:Fun)-[CALLS]->(callee:Fun)
    where callee.inst_count < 8
      and callee.call_count == 1
    action: inline(callee, into: caller)
}

// Find cross-bank trampolines needed:
query bank_trampolines {
    match (caller:Fun)-[CALLS]->(callee:Fun)
    where caller.bank != callee.bank
    action: insert_trampoline(caller, callee)
}

// Find dead stores:
query dead_stores {
    match (store:Inst{op: STORE})-[DEF]->(reg:Reg)
    where not exists (use:Inst)-[USE]->(reg)
    action: eliminate(store)
}

// Find constant propagation opportunities:
query const_prop {
    match (def:Inst{op: CONST, val: $v})-[DEF]->(reg:Reg)
          -[USE]->(use:Inst)
    where use.op in [ADD, SUB, AND, OR, XOR]
    action: fold(use, reg, $v)
}
```

This is essentially **Datalog over the MIR2 graph** — same engine,
different domain.

**Comparison to Cypher:**

| Feature | Cypher (Neo4j) | Our needs |
|---------|---------------|-----------|
| `MATCH (a)-[r]->(b)` | ✅ | ✅ same pattern |
| `WHERE` conditions | ✅ | ✅ simpler (no string ops) |
| `RETURN` | Results | Actions (rewrite) |
| `CREATE/DELETE` | Graph mutation | IR rewriting |
| Aggregation | `COUNT`, `SUM` | Cost estimation |
| Variable-length paths | `(a)-[*1..3]->(b)` | Overkill |
| Full text search | Overkill | Not needed |

We need ~30% of Cypher. A simpler syntax suffices:

```
// "Matchbox" — MinZ pattern query language
// Simpler than Cypher, focused on IR rewriting

pattern dead_load {
    %r = load %ptr
    no_use(%r)
    ---
    eliminate
}

pattern const_fold_add {
    %r1 = const $a
    %r2 = const $b
    %r3 = add %r1, %r2
    ---
    %r3 = const ($a + $b)
}

pattern inline_small {
    %r = call @fn($args...)
    where @fn.size < 8 and @fn.calls == 1
    ---
    inline @fn at %r
}
```

### Option C: Datalog for Both (unifying insight!)

**Key realization:** Datalog can serve BOTH use cases:

```
// User-facing: game data tables
piece_dx(0, 0, 0, 0).   // fact
piece_dx(0, 0, 1, 1).   // fact

// Compiler-internal: optimization rules
inline_candidate(F) :- callee(_, F), inst_count(F, N), N < 8.
dead_store(I) :- def(I, R), not(use(_, R)).
const_fold(I, V) :- arg(I, 0, R1), const(R1, A),
                     arg(I, 1, R2), const(R2, B),
                     V = A + B.
```

Same evaluation engine. Same syntax. Two domains:
1. **Compile-time data** — facts about game world → LUT arrays
2. **Compile-time analysis** — facts about IR → optimization decisions

**Soufflé** (the Datalog engine used by Facebook/Meta for program analysis)
proves this scales: millions of facts, sub-second evaluation. For Z80
programs with ~100 functions and ~1000 instructions, Datalog evaluation
would be instant.

### Three Levels of Declarative Optimization

Academic context: this is a well-researched space.
- **Datalog for program analysis:** Soufflé (Facebook Infer, DOOP), Bravenboer & Smaragdakis 2009
- **Term Rewriting Systems (TRS):** GCC match.pd, Cranelift ISLE
- **Equality Saturation:** egg, egglog — apply ALL rules to fixpoint, cost model picks winner

```
Current peephole:  local, greedy, order-dependent
TRS (Step 1):     local, all rules, cost resolves conflicts
Datalog (Step 2): global (whole CFG), fixpoint, analysis ≠ transformation
EqSat (Step 3):   global, optimal within rules, order-independent
```

Each level removes one limitation of the previous.

### Step 1: Declarative Peephole Rules (TRS) — ~1 week, NOW

Not a DSL file — just refactor Go rules into data structures:

```go
type Rule struct {
    Name    string
    Pattern []InstPattern
    Replace []InstTemplate
    Guard   func(ctx) bool
    Benefit int // T-states saved
}

var Z80Rules = []Rule{
    {
        Name:    "xor_zero",
        Pattern: []InstPattern{{Op: LD_A_IMM, Imm: 0}, {Op: AND_A}},
        Replace: []InstTemplate{{Op: XOR_A}},
        Benefit: 7,  // 11T → 4T
    },
    {
        Name:    "fold_push_pop",
        Pattern: []InstPattern{{Op: PUSH, Reg: "$rr"}, {Op: POP, Reg: "$rr"}},
        Replace: nil, // eliminate both
        Benefit: 21,
    },
    {
        Name:    "djnz",
        Pattern: []InstPattern{{Op: DEC_B}, {Op: JR_NZ, Label: "$L"}},
        Replace: []InstTemplate{{Op: DJNZ, Label: "$L"}},
        Benefit: 1,  // saves 1 byte
    },
}
```

67 rules → 67 struct literals. Engine walks rules, applies by benefit.
Low risk, high ROI. Testable: `assert applyRules(input) == expected`.

### Step 2: Embedded Datalog for Flow Analysis — ~2 weeks, AFTER modules

Small (~500 LOC) Datalog evaluator in Go. Semi-naive evaluation for fixpoint.
Queries over MIR2 CFG → result sets.

```datalog
% Liveness analysis
live(V, P) :- use(V, P).
live(V, P) :- live(V, Q), succ(P, Q), !def(V, P).

% Cross-bank trampoline detection (ADR-0021)
needs_trampoline(Caller, Callee) :-
    calls(Caller, Callee),
    bank(Caller, B1), bank(Callee, B2), B1 != B2.

% Inline candidates
should_inline(F) :-
    fun(F), size(F, S), S < 8, call_count(F, 1), !recursive(F).

% DJNZ candidates (our loop reversal proof!)
djnz_eligible(Loop) :-
    for_loop(Loop), counter_var(Loop, V),
    !used_in_body(V, Loop), !b_live_at(Loop).
```

Key insight: **analysis is separated from transformation**. Datalog answers
"what", Go code answers "how". Cleaner than current mixed approach.

### Step 3: Equality Saturation — FUTURE (academic publication level)

```
rewrite ld_a(0); and_a()  =>  xor_a()
    cost: 11T → 4T

rewrite add_hl_de(n)  =>  inc_hl() × n
    when: n <= 3
    cost: 21T → 6n T

// Saturation: apply ALL rules to fixpoint
// Cost model picks global minimum
```

EqSat guarantees global optimum (within rule set). Greedy peephole can't.
For Z80 with 7 registers and predictable ISA — EqSat could find optimizations
no greedy algorithm would. **This is PLDI-paper territory if MinZ does it.**

### Recommendation

**Phase 1 (now): TRS peephole refactor** — 1 week, immediate payoff.
Refactor 67 Go rules into struct literals + pattern engine.

**Phase 2 (next): `@facts` for users** — 2-3 days.
Declarative game data tables as Nanz annotation.

**Phase 3 (medium): Embedded Datalog** — 2 weeks.
Unified engine for user `@facts`/`@derive` AND compiler flow analysis.

**Phase 4 (future): Equality Saturation** — academic project.
Global optimal instruction selection. Publication-worthy.

---

## Part 4: Timeline & Priority Matrix

### Frontend Implementation Order

| # | Frontend | Time | Dependency | Deliverable |
|---|----------|------|------------|-------------|
| 1 | **Lizp** | 1-2 days | Lanz exists | `pkg/lizp/` — sugar over Lanz |
| 2 | **Pascal** | 2-3 days | PLM pattern | `pkg/pascal/` — hand-written parser |
| 3 | **C89** | 2-3 days | `modernc.org/cc/v4` (BSD-3) | `pkg/c89/` — lower.go only |
| 4 | **@facts** | 2-3 days | None | `@facts` annotation in Nanz parser |
| 5 | **Peephole DSL** | 2-3 days | None | `pkg/peephole/dsl.go` |
| | **Total** | **~12 days** | | **8 languages + peephole DSL** |

### The Dream: 8 Languages → 1 Backend

```
 Lizp ──┐
 Pascal ┤
 C89 ───┤
 Nanz ──┼── *hir.Module ── MIR2 ── PBQP ── Z80 asm
 Lanz ──┤
 PLM ───┤
 Datalog┤
 ObjC ──┘  (stretch goal: +1-2 days on C89)
```

### Languages That Collapse Into Nanz

These don't need frontends — Nanz already IS their static Z80 incarnation:

| Language | What Nanz took from it |
|----------|----------------------|
| Ruby | String interpolation `"#{x}"`, `fun`/`fn`, `.each { |x| }` |
| Crystal | Type inference, zero-cost abstractions |
| Swift | Value types, protocol-oriented design |
| Rust | Ownership concepts (manual), trait-like interfaces |
| Kotlin | Expression-oriented, null safety via `@error` |

### Languages That Need Runtime (impossible for Z80)

| Language | Why impossible |
|----------|---------------|
| Python | GC, dynamic typing, 256KB+ runtime |
| JavaScript | GC, prototype chain, JIT assumption |
| Lua | GC, tables, coroutines (but @lua works at compile-time!) |
| Gleam | ADTs, generics, Erlang-style concurrency |

---

## Appendix: License Compatibility

| Dependency | License | Compatible? |
|------------|---------|-------------|
| `modernc.org/cc/v4` | BSD-3-Clause | ✅ |
| Participle (Nanz parser) | MIT | ✅ |
| Go stdlib | BSD-3-Clause | ✅ |
| MinZ itself | (owner's choice) | ✅ |

No GPL/LGPL/copyleft dependencies. All frontends can use any license.
