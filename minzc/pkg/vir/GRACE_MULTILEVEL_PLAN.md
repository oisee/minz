# Grace Multi-Level Optimization Plan

## Done (2026-03-24)

### ASM Level (pipeline.go gracePass)
- [x] Conditional RET sinking (JR/JP cc → RET cc_inv)
- [x] Duplicate CP elimination
- [x] Dead LD elimination (LD r,X / LD r,Y → skip first)
- [x] Reverse-copy elimination (LD X,Y / LD Y,X → keep first)
- [x] Dead instruction before RET (DEC/INC/EX/LD)
- [x] Dead LD A before CALL
- [x] EX DE,HL / LD A,(HL) → LD A,(DE) fusion
- [x] LD pair,0 / ADD HL,pair → remove both
- [x] Redundant EX DE,HL pairs
- [x] EX DE,HL sandwich elimination (EX/instr/EX → swap refs)
- [x] JP threading (JP .a where .a only has JP .b → JP .b)
- [x] fuseAbsDiffASM (SUB/RET NC/NEG/RET pattern)
- [x] Grace reroll (--Osize): repeated LD×N/CALL → DJNZ loop + data table (1-8 args, 8/16 bit)

### Peephole Level (pipeline.go peepholeCleanup)
- [x] CALL/RET → JP (tail call)
- [x] Inline runtime (div8/mod8/mul8/div16/mod16/mul16)
- [x] LD r,r self-move removal
- [x] ADD A,0 / SUB 0 removal
- [x] LD A,0 → XOR A / CP 0 → OR A
- [x] INC/DEC cancel
- [x] Dead conditional branch (JP cc, next_label)
- [x] PUSH/POP different pair → LD halves
- [x] AND 0FFh → AND A / OR 00h → OR A

### VIR Level (isle.go ISLECombine)
- [x] load16_le fusion (FatFS ld_word)
- [x] store16_le fusion (FatFS st_word)
- [x] Multiply strength reduction (×2,3,4,5,8)
- [x] Identity/zero elimination
- [x] Constant multiply table (103 entries from GPU bruteforce)

### Z80 Patterns (z80.go)
- [x] store16 via DE without EX bracket (st16_de_hl_via_a, 24T vs 26T)

---

## Planned

### 1. MIR2 Level Reroll (HIGHEST PRIORITY)
**store×N + call → loop + data table at IR level**

Before codegen sees it, transform repeated patterns:
```
block entry:
  v1 = const "MAT-001"
  v2 = const 0
  v3 = const 11
  call table_set_str(v2, v3, v1)
  v4 = const "FERT"
  v5 = const 0
  v6 = const 11
  call table_set_str(v5, v6, v4)
  ... (×15 more)
```
Into:
```
block entry:
  v_table = addr_of _reroll_data
  v_count = const 15
  jmp loop_head(v_table, v_count)
block loop_head(table_ptr, remaining):
  br_if remaining == 0, exit, body
block body:
  v_str = load16(table_ptr + 0)
  v_row = load8(table_ptr + 2)
  v_col = load8(table_ptr + 3)
  call table_set_str(v_row, v_col, v_str)
  v_next = add(table_ptr, 4)
  v_dec = sub(remaining, 1)
  jmp loop_head(v_next, v_dec)
```

**Why MIR2?** Sees types (u8 vs u16), struct layouts, can prove safety. Z3 then allocates registers for the LOOP — which it handles perfectly (CFG solver).

### 2. MIR2 Level — Dead Argument Elimination
If a called function ignores param 3, don't emit the LD for arg 3 at call site.
Needs interprocedural analysis (check callee's contract).

### 3. MIR2 Level — Call Inlining (size-aware)
If function body ≤ 3 instructions, inline at call site.
Saves CALL+RET (27T). Only when called ≤ 3 times (avoid code bloat).

### 4. Module Level — Tail Merge
If functions F1, F2, F3 end with identical instruction sequences,
share the tail as a common block. F1/F2/F3 JP to shared tail.

### 5. Module Level — PFCCO-Aware Inlining
Inline if it eliminates register shuffles at call site.
Cost model: inline_size < call_overhead + shuffle_moves.

### 6. ASM Level — SMC Data Table (advanced)
Instead of IX-indexed loop, use self-modifying code:
patch LD immediates directly in the code for each iteration.
Faster than IX access (no DD prefix overhead).

### 7. ASM Level — LDI/LDIR for Block Copy
Detect memcpy-like patterns and use Z80's block transfer:
LDIR copies (HL)→(DE), BC bytes, auto-increment.

### 8. VIR Level — Coroutine Transform
Repeated pattern with different data → yield-based iterator.
Generator syntax: for x in table { process(x) }.

### 9. Cross-Function — Common Subexpression Sharing
Multiple call sites compute same value → hoist to shared helper.

### 10. ROM Level — Compressed Data Tables
For ROM-constrained targets: LZ-compress data tables,
emit tiny decompressor, decompress to RAM at startup.

---

## Priority Order
1. MIR2 reroll (biggest code size win, enables @screen optimization)
2. Dead argument elimination (easy, interprocedural)
3. Call inlining size-aware (medium effort, good perf win)
4. Tail merge (module level, code size)
5-10. Future work
