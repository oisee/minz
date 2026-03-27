# Session Wisdom: Sessions 10-11 (2026-03-27)

**Commits:** ~60 across main repo + VIR + z80-optimizer
**Releases:** v0.24.0 (One Compiler, 50 Years) with 16 assets

---

## Breakthroughs

### 1. VIR Default Backend Switch
LIR had register mismatch with assert harness: `add(3,4)=3` because harness used PBQP allocation but function was compiled via LIR (different registers). Fix: flip `--vir=true` default. VIR param locs match assert harness correctly.

### 2. mir2gpu — 5 Backend Targets
MIR2 → CUDA/OpenCL/Vulkan/Metal. 1284 LOC, 95% shared. 4/4 verified 256/256 on real hardware (NVIDIA, AMD RX 580, Apple M2). CLI: `mz program.nanz -b cuda|opencl|vulkan|metal`.

### 3. Frill = Nanz = Same ASM
Proven: both frontends produce identical Z80 asm AND identical CUDA kernel. MIR2 is the universal bridge. 8 frontends × 5 backends.

### 4. swap: RET (20:0 vs SDCC)
Z3-PFCCO assigns registers so swap params ARE already the return values. `swap() = RET` (1 instruction). SDCC = 20 instructions with stack push/pop. The calling convention IS the optimization.

### 5. abs_diff = 4 bytes (SUB/RET NC/NEG/RET)
Z3 found provably optimal sequence. Most humans write 6+ bytes. SDCC = 10 instructions.

### 6. C23 Struct → Tuple Promotion
ADR-0025: small structs automatically promoted to tuple returns. `DivResult divmod()` → returns (q=A, r=C) in registers. SDCC can't return structs at all — forces pointer-based output params (25+ instructions with stack).

### 7. GPU Auto-Detector
DetectGPUCandidates scans MIR2 loops: finds back-edges, checks bounded type, classifies map/filter/reduce. 18/18 tests. Loops → GPU kernels automatically.

### 8. shl-by-zero Fix (One Character)
`genShift` line 3729: `cv > 0` guard skipped cv=0 → count=1 → wrong shift. Remove `> 0`. Identity.

### 9. C Frontend Shift Narrowing
`narrow8` checked BOTH operands for u8, but shift count always promoted to int by cc. Fix: `narrow8Shift` checks LEFT operand only. u8 << N stays u8.

### 10. PBQPAlloc One-Liner
`SolverOptions{PBQPAlloc: combined}` — without it, VIR-failed functions had no PBQP code → missing __mul8 → assembly error. One line unlocked 3 Frill demos on Z80.

---

## Hard-Won Lessons

### VIR Zero Bugs — All Issues Were MIR2/C-Frontend
Every Z80 codegen bug found was in MIR2 or C frontend, NOT VIR:
- shl cv>0 guard (MIR2 genShift)
- shl u8→i16 promotion (C frontend narrowing)
- TermCondRet save-before-clobber (MIR2 codegen)
- LIR register mismatch (LIR, now deprecated)
- Missing PBQPAlloc pass-through (pipeline.go)

VIR+Z3 solver produces correct code given correct constraints.

### Assert Harness Must Match Backend
The Z80 assert harness reads param registers from AllocResult. If the function was compiled by a different backend (LIR vs VIR), registers don't match → wrong results. Fix: use the same backend for compilation and assertion.

### PFCCO Is The Optimization
Per-Function Calling Convention Optimization isn't just a calling convention — it's a whole-program optimization. Z3 considers all call sites simultaneously. When PFCCO works perfectly, the function body can be empty (swap=RET).

### Struct Return Promotion = Cross-Language Unification
C struct returns promote to tuples → same MIR2 as Nanz native tuples → same Z80 ASM. The abstraction gap between C and Nanz disappears at MIR2 level.

### GPU Backend Is 95% Shared
CUDA/OpenCL/Vulkan/Metal differ only in: kernel qualifier, thread ID, param syntax. The actual computation is identical. Universal MIR2 lowering → thin backend wrappers.

### Books Must Show Real ASM
The C23 book got dramatically better when we added real compiled output (MinZ vs SDCC side-by-side). Theory → practice. Readers can verify every claim.

---

## Research Findings

### Cross-ISA Arithmetic
z80-optimizer planning 6502 mul8: same abstract chains, different instruction materialization. Z80 vs 6502 comparison for all 254 constants.

### GPU as Exhaustive Verification Oracle
Compile same function to Z80 + GPU. Run all 256 inputs on GPU in parallel. Compare with Z80 emulator. Match = proven correct for entire u8 domain. Not testing — exhaustive proof.

### Approximate LUT Generators
GPU finds ~12-byte arithmetic sequences that generate 256-byte lookup tables at startup. For sin/cos/log2 where no compact inverse exists. @init metafunction concept.

### BCD Arithmetic GPU-Proven
BCD×2 = ADD A,A; DAA (8T). BCD+1 = INC A; DAA. 100's complement = NEG; DAA. All GPU-verified with proper H-flag model.
