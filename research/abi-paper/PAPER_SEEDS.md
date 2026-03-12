# Paper Seeds — Potential Separate Publications from Nanz

## Paper 1 (Current): Per-Function ABI on Irregular Register Files
**Venue:** CC or LCTES
**Status:** Phases 1-4 complete, Phase 5 (draft) next
**Core contribution:** PFCCO algorithm, O(n) optimality for DAGs, production implementation

---

## Paper 2: Iterator Chain Fusion for Register-Constrained Architectures
**Venue:** LCTES, CGO, or SCOPES

**Contribution:**
- Lambda-to-DJNZ fusion: high-level iterator chains (`map`, `filter`, `forEach`)
  compiled to tight Z80 loops with zero abstraction overhead
- Sequential pointer traversal vs C-style indexed access: `INC HL; LD A,(HL)`
  (13T) vs `LD L,i; LD H,0; ADD HL,base; LD A,(HL)` (~33T) — 2.5× per element
- Callback inlining: `CALL/RET` eliminated for small lambdas, body inlined
  directly into the DJNZ loop
- **Status:** 11/11 E2E verified, fusion optimizer live, ~5x perf overhead
  from register allocator bottleneck (memory-backed virtuals)

**Key examples:**
- `buf.forEach(|x| { s += x }, n)` → `DJNZ` loop, 38T/element vs SDCC's 88T
- `arr.iter().map(|x| x*2).filter(|x| x>5).forEach(|x| print(x))` — single pass
- `reduce` (fold) over array — accumulator in A, no stack frame

**Unique angle:** Zero-cost iterator abstraction on an 8-bit CPU with 7 registers.
Prior work on iterator fusion (stream fusion, deforestation) targets GC'd languages
with large register files. Nobody has shown this on Z80.

---

## Paper 3: Shadow Register Bank as Extended Calling Convention
**Venue:** SCOPES, CASES, or short paper at CC

**Contribution:**
- Systematic use of Z80 shadow registers (A', B', C', D', E', H', L') as
  part of the type system and calling convention
- ClassDWord: u32 values represented as `HL + H'L'` (main + shadow pair),
  switched via `EXX` (4T) — vs stack-based u32 in SDCC (~42T per load/store)
- ClassShadow: overflow 8-bit values placed in shadow bank (Tier 2 in cost table)
- Cost model: EXX pairs as "regions" — 8T overhead per shadow access
  (4T switch in + 4T switch out)

**Key examples:**
- u32 add: `ADD HL,DE; EXX; ADC HL,DE; EXX` — 4 instructions, 30T
  vs SDCC stack-based: ~80T (load from stack, add, store back)
- u24 support: upper byte of shadow pair always zero (3-byte load/store,
  no wasted space for 24-bit addressing on eZ80)

**Unique angle:** No published work systematically models shadow register banks
in a cost table for register allocation. Z80 shadow registers are typically
used only for interrupt handlers. Using them as a structured extension of
the calling convention is novel.

**Limitation:** Shadow registers can't be used across function calls (EXX
affects all three pairs simultaneously). The cost model must account for this.

---

## Paper 4: Self-Modifying Code as Calling Convention (TSMC)
**Venue:** LCTES or retro-computing workshop

**Contribution:**
- TRUE Self-Modifying Code: parameters passed by patching instruction immediates
  (`LD A, #imm` where #imm is overwritten by caller before CALL)
- Eliminates register setup entirely — caller writes directly to callee's code
- Patch tables: structured metadata for SMC call sites
- Smart patchable return sequences: NOP→RET metamorphism

**Key examples:**
- Function call: `LD (callee_param_a+1), A; CALL callee` — 0T param setup
  in callee (immediate already in instruction stream)
- Behavioral morphing: one function, multiple behaviors selected by patching

**Unique angle:** SMC is well-known in retro programming but never formalized
as a calling convention with structured patch metadata. The TSMC paper would
formalize the cost model, correctness conditions (no self-modifying in ROM),
and interaction with per-function ABI optimization.

---

## Paper 5: Compile-Time LUT Expansion via Range Annotations
**Venue:** Short paper or tool demo at CC/CGO

**Contribution:**
- Range annotations on parameters (`x: u8<0..255>`) enable the compiler to
  detect pure functions over bounded domains
- Automatic LUT generation: loop body → 256-byte lookup table
- Page-aligned table access: `LD H, table^H; LD L, x; LD A,(HL)` — 3 instructions

**Key example:**
- `popcount(x: u8<0..255>)` → 4 bytes code + 256 bytes LUT, 28T constant time
  vs loop: ~50 bytes, ~200T for popcount(255)

---

## Paper 6: Optimal Register Allocation for Multi-Return Functions
**Venue:** CC (extends Krause 2013)

**Contribution:**
- Extension of Krause 2013's optimal intraprocedural RA to handle
  multi-return functions on irregular architectures
- Z80 multi-return ABI: return[0]=HL, return[1]=DE, return[2]=B
- Contract optimizer integration: returns are part of the PFCCO cost function
- Current limitation: return classes NOT optimized yet (multi-return clobber bug,
  see ApplyContracts line 102-107). Fixing this unlocks the full potential.

**Key example:**
- `minmax(a,b) → (min, max)` in (HL, DE)
- `smaller(a,b) → min` = `EQU minmax` (0 bytes) — emergent from ABI alignment

---

## Publication Strategy

| Priority | Paper | Venue | Dependencies |
|----------|-------|-------|-------------|
| 1 | PFCCO (current) | CC/LCTES | None |
| 2 | Iterator Fusion | LCTES/SCOPES | Fix regalloc bottleneck first |
| 3 | Shadow Register ABI | SCOPES (short) | Can write now |
| 4 | TSMC Calling Convention | LCTES/workshop | Need to decouple from MIR1 |
| 5 | LUT Expansion | CC tool demo | Can write now |
| 6 | Multi-Return RA | CC | Fix clobber bug first |

Papers 1 + 3 could be combined into a single stronger paper: "Per-Function
ABI Optimization with Shadow Register Extensions for Irregular Architectures."
