# Report 093 — LIR Guided Register Allocation: PBQP + WFC Synthesis

**Date:** 2026-03-18
**Branch:** `feat/lir-backend`
**Commits:** 9 (d7e9a08..7ae6034)
**Status:** Production-matching codegen for leaf functions, correct calls with IXH spill, tail call optimization

---

## The Journey: From Wrong Code to Production-Quality

### The Problem

The LIR backend could lower MIR2 through ISLE combining and WFC constraint propagation, achieving 94.6% corpus convergence. But it generated **wrong code** for every non-trivial function. `add(a, b)` produced `ADD A, A` instead of `ADD A, C` — both parameters collapsed to the same register.

Three bugs, discovered in dependency order:
1. **Flat path ignored function params** — WFC had no idea which vreg should be in A vs C
2. **CALL instructions vanished** — OpCall was not lowered to any LIR pattern
3. **Destructive ALU overwrites** — `ADD A, r` kills the accumulator, but live values weren't saved

### The Solution: 9 Commits in One Session

#### Commit 1: Param Seeding
The flat codegen path called `SelectInstructions` (no param awareness) while the multi-block path had `SelectBlockInstructions` with pre-seeded param constraints. Fix: extract `f.Contract.Params` into `BlockParam` entries via `ContractParamsToBlockParams()` and use them everywhere.

**Result:** `add(a, b)` → `ADD A, B` (not `ADD A, A`)

#### Commit 2: OpCall Lowering
`translateCall()` converts MIR2 `OpCall` into argument setup moves + CALL instruction. Each arg move has `DstAllowed` constrained to the callee's param register class. Z80 patterns added: `call_u8` (dst=A), `call_u16` (dst=HL), `call_void`.

**Result:** `main() { putchar(7) }` → `LD A, 7; CALL putchar; RET`

#### Commit 3: WFC Clobber Pass
New propagation pass in WFC: for each CALL instruction, identifies vregs defined before and used after. These "live-across-call" vregs cannot be in registers that the call clobbers. On Z80, CALL clobbers all GPRs (A,B,C,D,E,H,L).

#### Commit 4: IXH/IXL as Call-Safe Locations
Z80 unique: IX/IY half-registers (undocumented DD/FD prefix instructions) are NOT clobbered by function calls. Added IXH, IXL, IYH, IYL as 8-bit locations in the Z80 descriptor with move patterns (`LD IXH, r` / `LD r, IXH`, 8T each).

When WFC's clobber pass finds a live-across-call vreg, it narrows the allowed set to `callSafeLocs()` = {IXH, IXL, IYH, IYL, spill slots}.

**Result:** `caller(x) { r = compute(x, 3); return r + x }` → x assigned to IXH, survives CALL

**Resource hierarchy (L0-L4):**

| Layer | Storage | Cost | Survives CALL |
|-------|---------|------|---------------|
| L0 | Same register | 0T | No |
| L1 | GPR move | 4T | No |
| L2 | IX/IY half | 8T | **Yes** |
| L3 | Shadow (EXX) | 4T/batch | **Yes** (future) |
| L4 | Memory | 13T | **Yes** |

#### Commit 5: Save-Before-Overwrite
`insertSaveBeforeOverwrite()` scans MIROps for destructive ALU operations. When an ALU op (e.g., `ADD A, r`) would overwrite a vreg that's still needed later, inserts an explicit copy move before the instruction. The saved copy goes to a non-A register (via `DstAllowed` excluding accumulator).

#### Commit 6: General Liveness Fix (Root Cause)
Found the real root cause: `findBestPattern()` was unioning `SrcLocs` across incompatible patterns. `add_a_r` (src0=A) and `inc_r` (src0=gpr8) got unioned to src0=gpr8, which hid the A-requirement from the setup move logic. Fix: filter INC/DEC patterns when src0 ≠ src1, since these only apply to self-increment.

**Result:** `compute2(a, b) = (a+b) + (a&b)` → 9 instructions, **correct result** (15 for inputs 10,3)

#### Commit 7: PBQP→WFC Hint Bridge (The Synthesis)
The breakthrough: PBQP (already running in production) sees the full interference graph and makes globally optimal register choices. WFC sees local Z80-specific constraints. By passing PBQP's allocation as **hints** to WFC's collapse, both work together:

```
PBQP: vreg→PhysLoc{Name:"C"} → pbqpToLIRHints() → AllocHints{vreg→2}
WFC:  pickPreferred(available, vreg) → prefer loc 2 (C) if available
```

**Result:** LIR output now **matches production byte-for-byte** for leaf functions:

| Function | Before (WFC alone) | After (PBQP→WFC) | Production |
|----------|-------------------|-------------------|------------|
| `add(a,b)` | `ADD A, B` | `ADD A, C` | `ADD A, C` |
| `add3(a,b,c)` | `ADD A,B; ADD A,C` | `ADD A,C; ADD A,D` | `ADD A,C; ADD A,D` |
| `mul_add(a,b,c)` | `ADD A,B; SUB C` | `ADD A,C; SUB D` | `ADD A,C; SUB D` |

#### Commit 8: Corpus + Peephole
- Added call patterns to RISC and CISC descriptors (were Z80-only)
- Peephole: skip `LD r, r` no-op moves where dst == src
- Corpus baseline: C89 94.6%, Nanz 84.6%, Lanz 100%, Lizp 86.0%

#### Commit 9: Tail Call Optimization
When the last instruction before RET is a CALL, replace with JP (saves 17 T-states):

```
Before: LD A, 65; CALL putchar; RET  (27T)
After:  LD A, 65; JP putchar          (10T, −63%)
```

---

## Architecture: Guided WFC

```
MIR2 Function
     │
     ├──→ PBQP (global interference graph, coarse allocation)
     │         ↓ AllocHints: vreg → preferred phys register
     │
     └──→ LIR Pipeline
              │
              ├── Bridge (MIR2 → MIROps + save-before-overwrite + call arg setup)
              ├── ISLE Combine (load16_le fusion, MUL strength reduction)
              ├── ISel (pattern matching, setup moves)
              ├── WFC (constraint propagation + PBQP-guided collapse)
              │     ├── forwardPass: def→use constraints
              │     ├── backwardPass: use→def constraints
              │     ├── vregConsistency: same vreg = same loc
              │     ├── clobberPass: call-safe narrowing (IXH/IXL)
              │     └── Collapse: pickPreferred(hints) → physical assignment
              ├── Peephole (LD r,r elimination)
              ├── Tail Call (CALL+RET → JP)
              └── Emit (template expansion → Z80 assembly)
```

**Key insight:** PBQP is the "Global Strategist" — it sees all interference edges and makes globally optimal choices. WFC is the "Local Tactician" — it verifies PBQP's choices against Z80-specific constraints (DD/FD prefix conflicts, accumulator destructiveness, IX half availability) and adjusts when needed. Neither alone is sufficient; together they produce production-quality code.

---

## Corpus Results

| Frontend | Functions (×3 machines) | Passed | Rate |
|----------|------------------------|--------|------|
| C89 | 720 | 681 | 94.6% |
| Nanz | 162 | 137 | 84.6% |
| Lanz | 9 | 9 | 100% |
| Lizp | 57 | 49 | 86.0% |
| **Total** | **948** | **876** | **92.4%** |

Remaining failures: 16-bit OpMove patterns (Z80-specific), OpMul (no hardware multiply).

---

## Roadmap

### Phase 2: EXX Shadow Stream (L3)
Z80's `EXX` instruction swaps BC↔BC', DE↔DE', HL↔HL' in 4 T-states. For functions with 3+ values live across a call, `EXX; CALL; EXX` (8T total) beats 3× PUSH/POP (63T) or 3× IXH save (48T).

WFC would model this as a 4th stream with group constraint: EXX regions are SESE (single-entry, single-exit), and callees must not use EXX internally.

**No existing Z80 compiler uses shadow registers for register allocation.** This would be a novel contribution.

### Phase 3: Iterative WFC
For functions exceeding the IX half spill budget (>4 live-across-call vregs), WFC needs iterative spill insertion:
1. Run WFC → detect contradiction (empty LocSet)
2. Insert spill moves (to stack/memory)
3. Re-run WFC with expanded instruction sequence
4. Converge in ≤7 iterations (Z80 has 7 GPRs)

### Phase 4: Full PBQP↔WFC Bidirectional Bridge
WFC feedback to PBQP: when WFC discovers that PBQP's choice creates a Z80-specific conflict (DD prefix, clobber chain), feed this back as an additional edge cost in PBQP's interference graph. Re-solve PBQP with updated costs. Converge in 2-3 rounds.
