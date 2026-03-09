# ADR-0010: Register-First Calling Convention with Mixed-Contract Support

## Status
Accepted

## Date
2026-03-06

## Context

MinZ's default calling convention is SMC (Self-Modifying Code): parameter values are
patched as immediates directly into the callee's instruction stream at each call site.
This was an innovative choice — 7-20T per parameter vs 44T+ for stack — and it works
well for leaf non-recursive functions called rarely.

However, with whole-program visibility (MinZ always rebuilds from source, like Go),
we can do better. The problems with SMC-as-default:

1. **Patch cost at every call site**: `LD (callee_param_imm), A` = 13T + 3 bytes per
   parameter, per call site. With register convention, parameters already in the right
   register cost 0T.
2. **Not re-entrant**: SMC functions cannot be called recursively or from interrupt
   handlers. Already disabled for recursive functions, but the constraint is invisible.
3. **ROM incompatible**: SMC requires code in RAM. Kills ROM-based targets and limits
   future portability.
4. **No interprocedural optimization**: SMC is opaque — callers don't know what registers
   the callee uses, so they can't arrange values optimally.
5. **5x register pressure overhead**: Virtual registers still spill to $F0xx memory because
   the convention doesn't help the allocator place values in real registers.

Krause (arXiv 2112.01397) empirically evaluated thousands of calling conventions for Z80
and found a 3-22% improvement (code size + speed) from switching SDCC to a register-based
fixed convention — enough that SDCC broke ABI compatibility to adopt it. MinZ can do more:
per-function conventions instead of a single fixed one.

The "register" calling convention already exists in MinZ codegen (caller + callee sides
implemented in `z80.go`) but is not the default. The missing piece is:
(a) making it default, and (b) a typed `FunctionContract` struct to hold per-parameter
location decisions including mixed SMC/register contracts.

## Decision

### 1. Change the default calling convention from SMC to "register"

**`ir/ir.go` — `NewFunction()`**: Set `IsSMCDefault: false, IsSMCEnabled: false`.

**`semantic/analyzer.go`**: When `CallingConvention == ""`, assign `"register"` instead
of setting `IsSMCDefault = true`.

SMC remains available via explicit `@abi(smc)` annotation and is still automatically
applied where it wins (see Mixed Contracts below).

### 2. Add `FunctionContract` to MIR

Replace `CallingConvention string` with a typed struct that supports per-parameter
location decisions:

```go
// LocationKind specifies how a single parameter or return value is passed.
type LocationKind uint8

const (
    LK_PhysReg  LocationKind = iota // Standard register (A, B, HL, DE, ...)
    LK_Stack                        // Stack slot (PUSH/POP)
    LK_Flag                         // CPU flag (CF, ZF — for boolean returns)
    LK_SMCPatch                     // SMC immediate patch (for loop-invariant constants)
)

// ConventionLocation describes where one value (param or return) lives.
type ConventionLocation struct {
    Kind        LocationKind
    Reg         PhysicalReg   // valid if Kind == LK_PhysReg
    Flag        FlagCondition // valid if Kind == LK_Flag (reuses CY/NC/Z/NZ)
    StackOffset int           // valid if Kind == LK_Stack
    Size        int           // 1 or 2 bytes
}

// FunctionContract is the typed calling convention for one function.
// nil = fall back to CallingConvention string (backward compatibility).
type FunctionContract struct {
    Params     []ConventionLocation // where each parameter arrives
    Returns    []ConventionLocation // where each return value goes
    Clobbers   RegisterSet          // registers destroyed by this function
    StackBytes int                  // total stack frame size
}
```

`ir.Function` gains a `Contract *FunctionContract` field. When non-nil, the codegen
uses the contract; when nil, falls back to the existing string-based path.

### 3. Mixed contracts: SMC per-parameter, not per-function

The `LK_SMCPatch` location kind enables mixed contracts:

```
fun render(x: u8, y: u8, color: u8)
  Contract.Params[0] = {LK_PhysReg, RegA}    // x: changes per frame
  Contract.Params[1] = {LK_PhysReg, RegB}    // y: changes per frame
  Contract.Params[2] = {LK_SMCPatch, ...}    // color: rarely changes → patch once
```

The caller patches SMC params once before a hot loop, loads register params each call.
This is superior to whole-function SMC (which patches all params each call) and
whole-function register (which wastes a register on a constant).

Mixed contracts are **designed now** but codegen implementation is deferred to Phase 4
(when the interprocedural synthesizer can identify which params benefit from SMC).
The `FunctionContract` struct accommodates them from day one.

### 4. Flag returns (per ADR-0008, extended scope)

ADR-0008 proposed flag returns scoped to iterator predicates. This ADR extends the
concept: `LK_Flag` in `FunctionContract.Returns` is the general mechanism. Any boolean-
returning function that is consumed primarily in conditional branches is a candidate.

The optimizer identifies these; codegen emits `SCF`/`CP`/`RET` instead of `LD HL,1/RET`.

### 5. IXH/IXL/IYH/IYL (configurable per target)

The undocumented Z80 register halves work on standard NMOS/CMOS Z80 (decades of ZX
Spectrum demoscene confirms this), and are documented on eZ80. Krause excluded them for
portability reasons; MinZ makes it configurable:

- `target.SupportsIXHalves bool` — true for Agon eZ80, optionally for ZX Spectrum
- Add `RegIXH, RegIXL, RegIYH, RegIYL` to `codegen.PhysicalReg` enum
- Alias constraints: IXH/IXL conflict with full IX; cannot mix
- Encoding cost model: +4T per instruction (prefix overhead) vs +11/10T for PUSH/POP
- Only assigned by the interprocedural synthesizer, never as default

### 6. Interprocedural contract synthesis (future pass)

A new `optimizer/cc_synthesizer.go` pass (Phase 4):

1. Use `RecursionDetector` call graph, collapse SCCs
2. Bottom-up: compute `Clobbers` from RegHints + transitive callee clobbers
3. Greedy assignment: assign params to registers free at all call sites
4. Identify SMC candidates: params that are loop-invariant at all call sites → `LK_SMCPatch`
5. Emit `FunctionContract` for each function
6. Codegen respects contracts

Use-analysis classification for pointer params (from docs/_in/claude-deep-01-b.md):
- `POINTER_A_ONLY` (all memory ops go through A) → assign to BC or DE, free HL
- `POINTER_MULTI_REG` (non-A loads/stores) → must be HL

## Implementation Phases

| Phase | What | Status |
|-------|------|--------|
| 1 | Change default to "register" in `NewFunction` + `analyzer.go` | **Now** |
| 2 | Add `FunctionContract` struct + `Contract *FunctionContract` to `ir.Function` | Next |
| 3 | `RegIXH/IXL/IYH/IYL` + alias rules in `register_allocator.go` | Soon |
| 4 | Implement ADR-0008 (`OpCallPredicate`, flag returns for iterators) | Soon |
| 5 | Interprocedural contract synthesizer (`cc_synthesizer.go`) | Later |
| 6 | Mixed-contract codegen (LK_SMCPatch per-param) | Later |

## Consequences

### Positive
- **0T parameter passing** for functions whose args are already in the right registers
- **Re-entrant by default** — register convention works across recursive calls, interrupts
- **ROM-compatible** — no self-modification required for standard functions
- **Interprocedural visibility** — contracts give the optimizer cross-function register info
- **SMC preserved** for the cases it wins: `LK_SMCPatch` for loop-invariant constants
- **Flag returns** eliminate bool→HL→test→branch overhead for predicates
- **Expected 10-30% improvement** on general code (comparable to Krause's SDCC result)

### Negative
- **Breaking change for programs depending on SMC-default behavior** — mitigated by
  `@abi(smc)` annotation for explicit opt-in
- **More complex codegen path** for mixed contracts (deferred)
- **IXH/IXL not portable** to SM83 (Game Boy) — gated behind target capability flag

### Neutral
- TRUE SMC (`UsesTrueSMC`) is unaffected — it has its own codegen path that bypasses
  the convention system entirely
- `@extern` functions are unaffected — they use explicit `TargetReg` per param
- `asm fun` bodies are unaffected — `IsAsm` flag bypasses convention codegen
- Recursive functions: already correctly handled (SMC disabled, convention used)
- Iterator DJNZ loops: already optimized via RegHints; this improves surrounding code

## Existing Infrastructure Reused

- `optimizer.RecursionDetector` — call graph + SCC detection (no changes needed)
- `ir.FlagCondition` (CY/NC/Z/NZ) — reused as `ConventionLocation.Flag`
- `codegen.RegisterSet` — reused for `FunctionContract.Clobbers`
- `z80.go loadParametersFromRegisters()` — already implements "register" callee side
- `z80.go` caller-side argument loading — already implements "register" caller side
- `ir.Function.UsedRegisters/ModifiedRegisters` — feeds into Clobbers computation

## References

- Krause, P.K. (2022). "Efficient Calling Conventions for Irregular Architectures."
  arXiv:2112.01397 — empirical Z80 calling convention study, 3-22% improvement
- Krause (CC 2013) — polynomial-time optimal register allocation for small irregular files
- Scholz & Eckstein (2002) — PBQP for irregular register allocation
- Brisk et al. (2007) — interprocedural SSA-based allocation
- docs/_in/claude-deep-01.md — full PBQP formulation and pipeline design
- docs/_in/claude-deep-01-b.md — A-register asymmetry, flag return ABI analysis
- [ADR-0008](0008-flag-based-boolean-abi-for-iterators.md) — flag returns for iterators
- [Report #026](../../reports/2026-03-06-026-Register_First_CC_Analysis_And_Plan.md) — code audit
