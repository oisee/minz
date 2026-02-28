# ADR-0009: Superoptimizer-Driven Peephole Rules

**Status:** Proposed
**Date:** 2026-02-28

## Context

MinZ's Z80 backend has two peephole optimization layers:

1. **MIR-level** (`peephole.go`, `mir_peephole.go`, `smart_peephole.go`) — 10+ patterns
   operating on IR instructions: constant folding, strength reduction, dead code elimination
2. **Assembly-level** (`assembly_peephole.go`) — 66 regex-based patterns operating on
   emitted Z80 assembly text: `LD A, 0` → `XOR A`, `ADD A, 1` → `INC A`, etc.

Both layers use **hand-written rules**. Each pattern was manually identified, implemented,
and tested. This approach has two fundamental limitations:

- **Incomplete coverage:** The Z80 instruction set has 406 opcodes (including undocumented).
  With 2-instruction sequences alone, that's ~165K combinations. Hand-writing is impractical
  beyond the most obvious patterns.
- **No correctness proof:** Each hand-written pattern is validated by unit tests with
  specific inputs, not exhaustive verification across all register states.

Meanwhile, the **z80-optimizer** project (`~/dev/z80-optimizer/`) has brute-force discovered
**602,008 provably correct** Z80 optimization rules by exhaustively testing all possible
register/flag inputs for every 2-instruction sequence. These rules are stored in
`rules.json` (102 MB, 3.4 MB gzipped).

### Rules Breakdown

| Savings | Count | Example |
|---------|-------|---------|
| 3 bytes | 1,212 | `SLA A : RR A` → `OR A` (-3B, -12T) |
| 2 bytes | 580,937 | `AND FFh : RR A` → `SRL A` (-2B, -7T) |
| 1 byte | 19,859 | `CPL : NEG` → `SUB FFh` (-1B, -5T) |

The rules cover transformations that no human would discover by inspection:
```
SLA A : RR A         →  OR A           ; Shift-left then rotate-right = identity + flags
SET 0, L : DEC HL    →  RES 0, L       ; Set bit 0, decrement = clear bit 0
ADD A, 80h : OR A    →  XOR 80h        ; Flip sign bit with correct flag behavior
SCF : ADC A, 00h     →  ADD A, 01h     ; Set carry + add-with-carry 0 = add 1
```

### Why Now

The z80-optimizer project is:
- Written in Go (same as MinZ)
- Produces JSON output compatible with programmatic consumption
- Includes `opReads()` / `opWrites()` / `areIndependent()` dependency primitives
  that enable reorder-aware matching
- Already verified against all 2^8 to 2^24 register states per rule (no false positives)

MinZ's assembly peephole already does iterative regex matching. Adding 602K rules
to a regex engine is impractical, but a **hash-lookup approach** is fast and simple.

## Decision

Integrate z80-optimizer rules into MinZ's peephole optimizer as a new pass:
**`SuperoptimizerPeepholePass`**, operating at the assembly level after existing
peephole patterns.

### Phase 1: Static Hash Lookup (immediate)

Load a curated subset of rules into a hash map keyed by the 2-instruction source pattern.

```go
// pkg/optimizer/superopt_peephole.go

type SuperoptRule struct {
    SourceAsm      string   // "SLA A : RR A"
    ReplacementAsm string   // "OR A"
    BytesSaved     int
    CyclesSaved    int
}

type SuperoptPeepholePass struct {
    rules map[string]SuperoptRule  // key = normalized source pattern
    mode  OptMode                  // OptSize | OptSpeed | OptBalanced
}

func (p *SuperoptPeepholePass) OptimizeAssembly(asm string) string {
    lines := strings.Split(asm, "\n")
    for i := 0; i < len(lines)-1; i++ {
        // Skip labels, comments, directives
        a := normalizeInstruction(lines[i])
        b := normalizeInstruction(lines[i+1])
        if a == "" || b == "" { continue }

        key := a + " : " + b
        if rule, ok := p.rules[key]; ok {
            // Apply rule: replace 2 lines with replacement
            lines[i] = "    " + rule.ReplacementAsm +
                fmt.Sprintf("    ; superopt: was %s (%dB, %dT saved)",
                    rule.SourceAsm, rule.BytesSaved, rule.CyclesSaved)
            lines[i+1] = ""  // Remove second instruction
        }
    }
    return strings.Join(lines, "\n")
}
```

**Rule loading:** At build time or startup, load `rules.json.gz` (3.4 MB) and build
the hash map. Only load rules where `bytes_saved >= 2` for the initial pass (582K rules)
to maximize impact. Rules with `bytes_saved == 1` are loaded separately for size-optimized
builds (`-Os`).

**Normalization:** Assembly instructions are normalized before lookup:
- Strip leading/trailing whitespace
- Normalize hex format: `0xFF` / `$FF` / `FFh` → `0FFh` (z80-optimizer canonical form)
- Preserve register names as-is (case-insensitive)
- Strip comments

### Phase 2: Reorder-Aware Matching (follow-up)

Port `opReads()` / `opWrites()` / `areIndependent()` from z80-optimizer to MinZ.
When two adjacent instructions don't match any rule, check if swapping them
(when safe) would expose a match.

```go
func (p *SuperoptPeepholePass) tryReorderedMatch(lines []string, i int) bool {
    if i+2 >= len(lines) { return false }
    a := parseInstruction(lines[i])
    b := parseInstruction(lines[i+1])
    c := parseInstruction(lines[i+2])

    // Can we swap b,c to match a rule on (a,c)?
    if areIndependent(b, c) {
        key := normalize(a) + " : " + normalize(c)
        if rule, ok := p.rules[key]; ok {
            // Swap b and c, then apply rule on (a, c_now_at_i+1)
            lines[i+1], lines[i+2] = lines[i+2], lines[i+1]
            applyRule(lines, i, rule)
            return true
        }
    }
    // Similarly try (b,a) after swap, (a,c) skip b, etc.
    return false
}
```

The dependency primitives use register masks:

```go
type regMask uint16
const (
    regA regMask = 1 << iota
    regF
    regB; regC; regD; regE; regH; regL
    regSP
)

func opReads(inst Instruction) regMask   // Which registers are read
func opWrites(inst Instruction) regMask  // Which registers are written
func areIndependent(a, b Instruction) bool {
    return (opWrites(a) & opReads(b)) == 0 &&  // no RAW
           (opReads(a) & opWrites(b)) == 0 &&   // no WAR
           (opWrites(a) & opWrites(b)) == 0      // no WAW
}
```

### Phase 3: Cost-Model Selection (future)

Add optimization mode to the CLI:

```
mz program.minz -Os          # Optimize for size (prefer bytes_saved)
mz program.minz -Ot          # Optimize for speed (prefer cycles_saved)
mz program.minz -O2          # Balanced (default)
```

When multiple rules match the same source, select by mode:
- `-Os`: maximize `bytes_saved`
- `-Ot`: maximize `cycles_saved`
- `-O2`: maximize `bytes_saved * 2 + cycles_saved`

### Integration Point

The pass runs **after** the existing `AssemblyPeepholePass` in the Z80 backend pipeline:

```
Z80 codegen → AssemblyPeepholePass (66 hand-written) → SuperoptPeepholePass (602K proven)
```

Hand-written patterns run first because they handle MinZ-specific idioms (SMC markers,
label-aware transformations, multi-line patterns) that the superoptimizer doesn't know about.
The superoptimizer pass then catches the remaining 2-instruction patterns.

### Safety

The assembly-level pass inherits existing safety constraints:

- **Skip labels:** Never optimize across label boundaries (control flow merge points)
- **Skip SMC markers:** Lines containing `_imm`, `PATCH`, `SMC` are not touched
- **Skip directives:** `ORG`, `DB`, `DW`, `EQU`, `END` are skipped
- **Flag-safe:** z80-optimizer rules are verified with full flag state — they're
  guaranteed to produce identical flags. This is *stronger* than hand-written patterns
  which often ignore flag side effects.
- **No false positives:** Every rule was verified by exhaustive state enumeration
  (512 to 33M register combinations per rule). The only way a rule can be wrong is
  if the z80-optimizer's CPU model is wrong — and that model is independently tested.

### Rule Storage

Three options for shipping rules with the compiler:

| Option | Size | Startup | Notes |
|--------|------|---------|-------|
| **A: Embedded Go map** | ~15 MB binary increase | Instant | `go:embed` + gob/msgpack |
| **B: External .json.gz** | 3.4 MB file | ~200ms load | Separate file, `--superopt-rules` flag |
| **C: Compiled Go code** | ~40 MB binary increase | Instant | `var rules = map[string]Rule{...}` |

**Recommended: Option B** for initial implementation. External file allows rule
updates without recompiling. Can move to Option A for release builds.

## Consequences

### Positive

- **602K proven rules** vs 66 hand-written — orders of magnitude more coverage
- **Zero false positives** — exhaustive verification is stronger than unit tests
- **Automatic discovery** of non-obvious optimizations humans would never find
- **Measurable impact** — each rule reports exact bytes and T-states saved
- **Composable** — hand-written patterns handle MinZ idioms, superoptimizer catches Z80 patterns
- **Future-proof** — when z80-optimizer adds length-3 rules (planned via GPU search),
  MinZ gets them for free by reloading the rules file
- **Reorder-aware matching** (Phase 2) exposes patterns hidden by instruction scheduling,
  multiplying the effective rule count significantly

### Negative

- **Rule file size** — 102 MB JSON (3.4 MB gzipped) is large for distribution.
  Need to decide on embedding vs. external file strategy.
- **Startup cost** — Loading 602K rules takes ~200ms. Acceptable for compilation but
  not for REPL. Can lazy-load or use binary format.
- **Hash collisions** — Normalization must be exact. Different hex formats (`$FF` vs `0FFh`)
  or spacing will cause misses. Need robust normalizer.
- **Two optimization systems** — Hand-written + superoptimizer adds maintenance surface.
  Clear separation (MinZ idioms vs. Z80 instruction pairs) mitigates this.
- **Undocumented opcodes** — z80-optimizer includes `SLL` and other undocumented Z80 ops.
  Rules using these should be filtered unless `--target` supports them.

### Neutral

- Does not replace hand-written patterns — complements them
- Does not change MIR-level optimization — only assembly level
- Rules are read-only — MinZ never modifies `rules.json`
- No impact on compilation without `rules.json` present (graceful degradation)

## Alternatives Considered

### A: Replace All Hand-Written Patterns with Superoptimizer Rules

Rejected. Hand-written patterns handle multi-line sequences (3+ instructions),
MinZ-specific idioms (SMC, shadow registers), and context-dependent optimizations
(label awareness) that the current superoptimizer doesn't cover. The two systems
are complementary, not competing.

### B: Run Superoptimizer at MIR Level Instead of Assembly Level

Rejected for Phase 1. MIR instructions don't map 1:1 to Z80 opcodes — one `OpAdd`
might generate `LD A, C : ADD A, B` or `ADD HL, DE` depending on types. Matching
at MIR level would require a translation layer. Assembly-level matching is simpler
and catches patterns introduced by the codegen itself.

Future consideration: A MIR-level version could optimize *before* register allocation,
which would expose different patterns. But this requires mapping MIR ops to Z80
instruction equivalence classes, which is a research problem.

### C: Use STOKE-Style Stochastic Search for Longer Sequences

Deferred to Phase 3. The z80-optimizer's roadmap includes MCMC-based stochastic
search for length 4-10 sequences. This would find much larger optimizations but
with probabilistic guarantees instead of exhaustive proof. Worth pursuing after
the deterministic rules are integrated.

### D: GPU-Accelerated Search for Length-3 Rules

Deferred. The z80-optimizer plans to port the executor to CUDA for brute-force
length-3 search (~19 minutes on 2x RTX 4060 Ti vs. estimated weeks on CPU).
This would produce millions more rules. The integration architecture in this ADR
already supports arbitrary rule counts — just reload the file.

## Implementation Plan

| Phase | What | Effort | Impact |
|-------|------|--------|--------|
| 1 | Hash-lookup pass + rule loader + normalizer | 2-3 days | Immediate: 602K rules applied |
| 2 | `opReads`/`opWrites`/`areIndependent` port + reorder matching | 3-5 days | 3-10x more pattern matches |
| 3 | Cost-model `-Os`/`-Ot` selection + metrics reporting | 1-2 days | User-tunable optimization |
| 4 | Embed rules in binary (go:embed + binary format) | 1 day | Zero-config deployment |

## References

- [z80-optimizer](../../../z80-optimizer/) — Superoptimizer project (Go, same author)
- [z80-optimizer/docs/NEXT.md](../../../z80-optimizer/docs/NEXT.md) — GPU + STOKE + reorder roadmap
- [Massalin 1987](https://dl.acm.org/doi/10.1145/36206.36194) — Original superoptimization paper
- [pkg/optimizer/assembly_peephole.go](../../minzc/pkg/optimizer/assembly_peephole.go) — Current 66-pattern peephole
- [pkg/optimizer/smart_peephole.go](../../minzc/pkg/optimizer/smart_peephole.go) — MIR reorder + peephole
- [ADR-0008: Flag-Based Boolean ABI](0008-flag-based-boolean-abi-for-iterators.md) — Related: flag-aware optimization
