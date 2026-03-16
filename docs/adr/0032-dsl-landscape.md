# ADR-0032: DSL Landscape for MinZ Compiler Backend

**Status:** Draft
**Date:** 2026-03-16

## Problem

We need multiple DSLs for different compiler tasks. The question is which
existing DSLs to adopt/adapt vs. inventing our own.

## DSL Inventory — What We Actually Need

### DSL 1: Instruction Selection Rules
**Task:** MIR2 op + operand constraints → target instruction(s)
**Best fit:** ISLE (Cranelift's term-rewriting DSL)

```lisp
;; Z80: lower 8-bit add
(rule (lower (add ty:u8 x y))
  (z80_add_a (put_in_a x) (put_in_gpr8 y)))

;; Z80: lower 8-bit add with immediate 1 → INC
(rule 1 (lower (add ty:u8 x (const 1)))
  (when (same_loc x (result_loc)))
  (z80_inc (put_in_gpr8 x)))

;; 6502: lower 8-bit add (must go through A)
(rule (lower (add ty:u8 x y))
  (m6502_adc (put_in_a x) (put_in_azpxy y)))
```

Why ISLE:
- Cranelift already uses block params (like our MIR2)
- Term rewriting is natural for tree-shaped IR lowering
- Compiles to efficient decision tree — no runtime interpreter needed
- Priority system (rule N) handles overlapping patterns cleanly
- Proven in production (Wasmtime processes billions of instructions)

### DSL 2: Target Machine Description
**Task:** Describe registers, constraints, instruction encoding
**Best fit:** TableGen-inspired declarative records

```
// z80.td — target description
def A  : Reg<8, "A",  acc>;
def B  : Reg<8, "B",  gen>;
def HL : RegPair<16, "HL", ptr, [H, L]>;
def IX : RegPair<16, "IX", ixy, [IXH, IXL]>;

def ADD_A_r : Inst<"ADD A, {src}", cost=4, bytes=1> {
  let dst = A;
  let src = GPR8;
  let clobbers = [F];
}

// DD prefix constraint
def : Forbidden<IXH, (ix_indirect IX)>;
def : Forbidden<H, IXH>;  // LD H,IXH = NOP
```

Why TableGen-style:
- Declarative, not imperative
- Can generate Go structs, test cases, documentation from same source
- LLVM proved it works for 20+ target architectures

### DSL 3: Peephole / Rewrite Rules
**Task:** Optimize instruction sequences (post-selection)
**Best fit:** egg-inspired equality saturation OR simple pattern→replacement

For simple 2-instruction peepholes (superoptimizer integration):
```
// Simple pattern → replacement (from superoptimizer)
rewrite "SLA A; RR A" → "OR A"     // -3B, -12T
rewrite "AND 0xFF; RR A" → "SRL A" // -2B, -7T
rewrite "CPL; NEG" → "SUB 0xFF"    // -1B, -5T
```

For IR-level rewrites (equality saturation):
```
// e-graph rules — explore all simultaneously, pick cheapest
(rewrite (sub x x) 0)
(rewrite (add x 0) x)
(rewrite (mul x 1) x)
(rewrite (sub (add x y) y) x)  // cancel add-sub
(rewrite (neg (sub a b)) (sub b a))  // neg of sub = swap
```

Why two levels:
- Assembly peepholes are string-based (superoptimizer output format)
- IR rewrites need semantic understanding (types, liveness, aliases)
- egg handles the "which order to apply rules" problem automatically

### DSL 4: Constraint Rules (Datalog)
**Task:** Express target invariants that must hold at all times
**Best fit:** Datalog (already implemented as FactDB)

```prolog
% Already in rules.go — keep this
forbidden(IXH, ix_indirect).
alias(HL, H). alias(HL, L).
acc_only(add8, A).
pair_only(add16, BC, DE, HL, SP).
clobber(add8, F).
sets_flag(srl, ZF). sets_flag(srl, CF).
preserves_flag(inc, CF).
```

Why Datalog:
- Facts + rules = complete constraint system
- Query with wildcards: `forbidden(_, ix_indirect)` → all DD conflicts
- Extensible: add new target = add new facts
- Simple semantics: any CS student can read/write it

### DSL 5: Convergence Test Specification
**Task:** Define what "correct" means across all backends
**Best fit:** Assert DSL (already exists in .nanz files)

```nanz
// Same file runs on MIR2-VM, LIR-VM(×4), Z80, QBE, C
fun fib(n: u8) -> u16 { ... }
assert fib(10) == 55              // all backends
assert fib(10) == 55 via mir2     // MIR2 VM only
assert fib(10) == 55 via lir      // all LIR VMs
assert fib(10) == 55 via z80      // Z80 binary
assert fib(10) == 55 via qbe      // native (QBE)
```

Why keep existing:
- Already works for MIR2 and Z80
- Just extend `via` to include `lir`, `lir:risc32`, `lir:cisc`, etc.

## Summary: 5 DSLs, Each Best-in-Class

| DSL | Purpose | Inspiration | Status |
|-----|---------|-------------|--------|
| **ISLE-like** | Instruction selection | Cranelift ISLE | 📋 Design |
| **TableGen-like** | Machine description | LLVM TableGen | 📋 Design |
| **Pattern rules** | Assembly peephole | Superoptimizer | ✅ ADR-0009 |
| **Datalog** | Target constraints | Datalog/Prolog | ✅ Implemented |
| **Assert** | Convergence tests | Nanz assert | ✅ Implemented |

+ WFC solver that USES rules from all 5 DSLs to find optimal solutions.

## Non-Goals

- Not implementing a full Datalog engine (no recursion, no stratification)
- Not implementing a full e-graph library (use simple rewrite for now)
- Not parsing ISLE syntax exactly (adapt to Go, keep the ideas)
- Not supporting all MLIR features (we're targeting 8-bit CPUs, not ML)
