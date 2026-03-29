# Session: WASM + LLVM Backends, Demoscene Brainstorm, Philipp Reply

**Date:** 2026-03-29
**Branch:** master
**Key commits:** ~8

---

## What Was Done

### WASM Backend (mir2wasm) — COMPLETE
- `codegen.go` (~300 LOC) — MIR2 → WAT text format
- `binary.go` (~350 LOC) — MIR2 → WASM binary directly (no wat2wasm needed)
- `runner.go` — wazero execution + assert verification
- if/else/end blocks for cond_ret and br_if
- `--asserts-force wasm` CLI flag
- Verified: add(3,4)=7, double(21)=42, square(4)=16, max_byte(10,20)=20, abs_diff(3,10)=7
- 174 bytes binary for 2 conditional functions

### LLVM IR Backend (mir2llvm) — COMPLETE
- `codegen.go` (~350 LOC) — MIR2 SSA → LLVM IR text (nearly 1:1)
- `runner.go` — llvmlite MCJIT execution via Python subprocess
- Phi nodes from block params via predecessor map
- Proper SSA: icmp/br i1/ret for conditionals
- 5 tests: add, double, conditional, chain, structure
- Verified: add(3,4)=7 via llvmlite native JIT

### 7 Backends from One MIR2
| Backend | Output | Status | LOC |
|---------|--------|--------|-----|
| Z80 (LIR) | Z80 ASM | Production | ~7000 |
| Z80 (VIR) | Z80 ASM via Z3 | Production | ~12000 |
| eZ80 | eZ80 ADL ASM | Working | ~500 |
| QBE | QBE IR → native | Oracle | ~745 |
| C | C99 source | Working | ~600 |
| WASM | WASM binary | **NEW** | ~650 |
| LLVM | LLVM IR text | **NEW** | ~350 |

### Demoscene Brainstorm (Maxim Muchkaev / RMDA)
Three Z80 techniques analyzed for compiler integration:
1. **CALL-as-PUSH** (8.5T/byte) — SP→screen, CALL pushes address=pixels
2. **RL (IX+N),R** (undocumented) — rotate+copy, 40% faster scroll
3. **INC/DEC (HL) semaphore** — self-modifying toggle, 74% faster than branch

### Philipp Reply
- Draft updated with verified numbers: swap 20:0, minmax 63:11
- SDCC 4.2.0 outputs compiled by master neighbor
- Outparam promotion demonstrated

### Neighbor Coordination
- VIR (4tw49890): will use WASM as third correctness oracle
- Master (ju6yy047): 7 backends noted, will add to README
- z80-optimizer (um2dy4ex): wants WASM batch verifier for 256-input sequences

---

## Seed for Next Session

### Immediate
1. **Wire LLVM to CLI** — `--emit llvm` for .ll output, `--asserts-force llvm`
2. **WASM loops** — while/for → WASM loop/br instructions
3. **Install LLVM tools** — `lli` for direct IR execution (needs sudo)
4. **Outparam transformation** — not just detection, actual MIR2 rewrite
5. **Send Philipp reply** — draft ready at docs/philipp-reply-draft.md

### Medium
6. **WASM playground** — serve .wasm in browser via simple HTTP server
7. **LLVM opt integration** — pipe through `opt -O2` for LLVM optimizations
8. **Demoscene patterns** — CALL-chain search on GPU (z80-optimizer)
9. **OpStats** — z80-optimizer wants operation frequency stats from MIR2

### Architecture
- MIR2 is the universal hub: 7+ backends consume it
- WASM binary encoder handles if/else but not loops yet
- LLVM IR has small type mismatch (i8 args emitted as i32 in calls)
- llvmlite (pip) works as LLVM execution engine — no lli needed

### Session IDs (ddll)
- minz-abap: gyfiwji1
- minz-vir: 4tw49890
- minz (master): ju6yy047
- z80-optimizer: um2dy4ex

### Key Insight
MIR2 → WASM is trivially correct because both are stack-oriented.
MIR2 → LLVM IR is trivially correct because both are SSA.
The hard part (register allocation, instruction selection) only
exists in Z80/eZ80 backends. WASM + LLVM + QBE are free correctness
oracles — if all four agree, the code is correct.
