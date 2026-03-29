# Session Wisdom: Session 16 (2026-03-29)

## Breakthroughs

### Scalar Operator Overloading for Primitives
`fun *(a: u8, b: u8) -> u16` now fires for scalar types, not just structs.
Multi-dispatch: `opTable` stores `[]opOverload`, `matchOpOverload()` does
exact (lhsTy, rhsTy) match for scalars, legacy struct match as fallback.
funcName mangling: `op_mul_u8_u8` for scalars, `op_mul` for structs.

### GPU-Optimal mul16 in Codegen — 7.7× Speedup
254 GPU-proven HL×K→HL sequences inlined at `CALL __mul16`.
Key fix: `resolveRegValue` now handles pair loads (`LD BC, N` → B=N>>8, C=N&0xFF).
`resolveDEValue` tracks through `EX DE, HL` by resolving HL before exchange.
×3=26T, ×10=48T, ×100=92T vs generic 200T loop.

### ^ is Pointer Dereference, NOT XOR!
In Nanz: `expr^` = postfix pointer deref (`LoadExpr`), `xor` keyword = bitwise XOR.
`a ^ b` parses as: load from address `a`, then orphan `b`. Causes OOB in MIR2 VM.
Fix: use `a xor b` for bitwise XOR, `a & b` for AND, `a | b` for OR.

### u32 Arithmetic — DEHL Convention (Verified Optimal)
ADC HL,rr EXISTS! ED 4A/5A/6A/7A, 15T. Also SBC HL,rr (ED 42/52/62/72).
SHL32: ADD HL,HL / EX DE,HL / ADC HL,HL / EX DE,HL = 34T (proven optimal)
SHR32: SRL D / RR E / RR H / RR L = 32T (proven optimal)
ADD32 from stack: POP BC / ADD HL,BC / POP BC / EX / ADC HL,BC / EX = 54T
NEG32: LD A,0 (not XOR A!) to preserve CY between bytes = 57T

### sat_add8 = 4 Instructions, 16T (GPU-Proven Masterpiece)
ADD A,B; LD C,A; SBC A,A; OR C — overflow→0xFF|anything=0xFF, no overflow→0x00|sum=sum.

### SHA-256 on Z80 — 808 Bytes, 58ms/block
15 functions, all asserts pass. ROTR tricks: ROTR8=free (byte rename),
ROTR16=EX DE,HL (4T). Full block: ~202K T-states = 58ms @3.5MHz = 17 blocks/sec.

## GPU Tables Received (z80-optimizer)
- mul8 A×K→A: 254/254 (existing)
- mul16 HL×K→HL: 254/254 (NEW, integrated in codegen)
- div8 A÷K→A: 254/254 (loaded, codegen wiring pending)
- mod8 A%K→A: 254/254
- divmod8 A÷K→A,B: 254/254
- u32 ops: 13 operations (SHL32, SHR32, ADD32, NEG32, etc.)
- sign8: 43T, sat_add8: 16T, sat_sub8: 20T
- arith16: abs16 44T, neg16 27T, min16/max16 41T
- SHA-256 round analysis

### div8 v3 carry_compare — GPU-Discovered, Not In Any Textbook
For K≥128, quotient always 0 or 1:
`OR A; LD B,(256-K); ADC A,B; SBC A,A; AND 1` = 5 ops, 26T, branchless.
ADC overflow detects A≥K. SBC A,A materializes carry. AND 1 clamps.
Verified: 32768/32768 exhaustive. avg div8: 154T→79T (−49%).
Three-level validation found what each alone missed:
1. Analytical (Hacker's Delight) → v1 baseline
2. Compositional search (preshift) → v2 −12%
3. GPU brute-force → v3 −49% (carry_compare discovery)

### VIR IntrinsicTable Architecture (from 4tw49890)
`ISLE(isel) → IntrinsicTable(strength reduction) → Z3(regalloc)`
intrinsics.go pushed same day: tryConstDiv (carry_compare), tryConstMod (AND mask).
Pattern: OpAsmBlock with GPU-precomputed ops[], clobbers from table, Z3 allocates around.

### Cross-Team Productivity
5 sessions coordinated via dedelulu in one day:
- z80-optimizer: 7 JSON files, ~800 entries, carry_compare discovery
- minz-vir: intrinsics.go same-day turnaround
- minz-abap: 42 LLVM native binaries, FatFS→ABAP on SAP
- antique-toy: FatFS source sharing

## Known Issues
- ~~div8 codegen not wiring~~ → FIXED by 4tw49890's intrinsics.go (8cfba219)
- MIR2 VM: struct return field access causes OOB (heap too small for struct pointers)
- MIR2 VM: `u8(expr)` cast may trigger unexpected load
- fun/ examples are self-contained demos, NOT importable stdlib modules yet

## Nanz Operator Cheat Sheet
| Symbol | Meaning | Example |
|--------|---------|---------|
| `^` | pointer deref (postfix) | `ptr^` = *ptr |
| `xor` | bitwise XOR | `a xor b` |
| `&` | bitwise AND | `a & b` |
| `\|` | bitwise OR | `a \| b` |
| `+` `-` `*` `/` `%` | arithmetic | standard |
| `==` `!=` `<` `>` `<=` `>=` | comparison | standard |
