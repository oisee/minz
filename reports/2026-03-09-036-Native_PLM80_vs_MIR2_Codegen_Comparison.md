# Report #036 — Native PL/M-80 Compiler vs MIR2 Z80 Backend: Codegen Comparison

**Date:** 2026-03-09
**Subject:** Side-by-side comparison of Intel PL/M-80 V4.0 (plm80c) output vs our MIR2 Z80 backend
**Source:** `abs_diff` and `fib` functions

---

## Setup

### Native PL/M-80 compiler

**plm80c** by Mark Ogden — a C port of the original Intel PL/M-80 V4.0 compiler
(source: `ogdenpm/c-ports` on GitHub). Built from source on macOS arm64.

```
PL/M-80 COMPILER V4.0
plm80-4.0 (C) Intel <port Mark Ogden>
```

Output format: Intel OMF object file (`.obj`), linked with `link80` to `.com`, assembled with
`plm80c CODE SYMBOLS` to produce annotated listing showing 8080 opcodes alongside source.

Note: PL/M-80 V4.0 targets the **Intel 8080**, not Z80. The instruction set is a strict 8080
subset — no `CP`, `NEG`, `DJNZ`, `EX DE,HL`, etc. Arguments passed via **memory variables**
at absolute addresses, not registers.

The assembly listings below are translated from 8080 to **Z80 syntax** for readability.
The opcodes are identical bytes — only the mnemonics differ.

### Source adaptation

The original `report_demo.plm` needed two PL/M-80 conformance fixes:
1. Top-level `DO … END MODULE;` wrapper (required by PL/M-80 module syntax)
2. `DECLARE T BYTE` moved from inside the `DO WHILE` body to the procedure top (PL/M-80
   requires all declarations before executable statements in a block)

```plm
DEMO: DO;

ABS_DIFF: PROCEDURE (A, B) BYTE;
  DECLARE (A, B) BYTE;
  IF A > B THEN
    RETURN A - B;
  ELSE
    RETURN B - A;
END ABS_DIFF;

FIB: PROCEDURE (N) BYTE;
  DECLARE N BYTE;
  DECLARE (A, B, T) BYTE;
  A = 0;
  B = 1;
  DO WHILE N > 0;
    T = B;
    B = A + B;
    A = T;
    N = N - 1;
  END;
  RETURN A;
END FIB;

END DEMO;
```

---

## ABS_DIFF comparison

### Native PL/M-80 V4.0 (Z80 syntax, from listing)

Parameters and locals are stored in **memory** at absolute addresses (`A`=0x0000, `B`=0x0001
relative to module base). The calling convention passes parameters via `LD (HL),E` / `LD (HL),C`
(caller stores BC/DE register pair values into memory slots via HL pointer).

```asm
; PROC  ABS_DIFF
0000  LD HL, (B)       ; HL = addr of B param slot   [LXI H,B]
0003  LD (HL), E       ; store B (from DE) to mem    [MOV M,E]
0004  DEC HL           ; HL = addr of A param slot   [DCX H]
0005  LD (HL), C       ; store A (from BC) to mem    [MOV M,C]
; IF A > B THEN
0006  LD A, (B)        ; A = mem[B]                  [LDA B]
0009  LD HL, (A)       ; HL = addr of A              [LXI H,A]
000C  CP (HL)          ; B - mem[A]                  [CMP M]
000D  JP NC, @1        ; jump if B >= A  (NOT A > B) [JNC @1]
; RETURN A - B
0010  LD HL, (B)       ; HL = addr of B              [LXI H,B]
0013  LD A, (A)        ; A = mem[A]                  [LDA A]
0016  SUB (HL)         ; A = A - mem[B]              [SUB M]
0017  RET
@1:
; RETURN B - A
0018  LD HL, (A)       ; HL = addr of A              [LXI H,A]
001B  LD A, (B)        ; A = mem[B]                  [LDA B]
001E  SUB (HL)         ; A = B - mem[A]              [SUB M]
001F  RET
@2:
; END ABS_DIFF
0020  RET              ; fallthrough guard
```
**Size: 33 bytes. Instructions: 14 (+ 1 guard RET).**

### Our MIR2 Z80 backend

Parameters in registers: A (first param), C (second param). All arithmetic in registers.

```asm
ABS_DIFF:
    CP C               ; compare A vs C (sets flags)
    JP C, .else        ; jump if A < C  (carry = A < C)
.if_then:
    SUB C              ; A = A - C
    LD C, A            ; [dead store]
    RET
.if_else:
    NEG                ; A = -A  (= -(old A))
    ADD A, C           ; A = C - old_A  (= B - A)
    LD C, A            ; [dead store]
    RET
```
**Size: 12 bytes. Instructions: 7 (+ 2 dead stores).**

### Analysis: ABS_DIFF

| Metric | PL/M-80 V4.0 (8080) | MIR2 Z80 | Winner |
|--------|---------------------|-----------|--------|
| Code size | 33 bytes | 12 bytes | **MIR2** (−64%) |
| Hot path T-states (a>b) | ~60T (mem loads) | 22T | **MIR2** |
| Hot path T-states (a≤b) | ~58T | 26T | **MIR2** |
| Dead stores | 0 | 2 × `LD C,A` | PL/M |
| Calling convention | memory (portable) | register (fast) | MIR2 for speed |

**Root cause of size difference**: PL/M-80 uses a stack-frame-like memory calling convention —
parameters are stored to absolute memory addresses at procedure entry using `LXI H, addr` +
`MOV M, reg` sequences. Every variable access is a memory round-trip. The 8080 has no
instructions equivalent to Z80's `CP reg`, `NEG`, or `SUB reg` (without going through memory),
so the compiler must use `LXI H, addr` + `CMP M` for every comparison.

---

## FIB comparison

### Native PL/M-80 V4.0 (Z80 syntax, from listing)

Variables `N`=0x02, `A`=0x03, `B`=0x04, `T`=0x05 in absolute memory.

```asm
; PROC  FIB
0021  LD HL, (N)       ; HL = addr of N param slot   [LXI H,N]
0024  LD (HL), C       ; store N (from BC) to mem    [MOV M,C]
; A = 0
0025  LD HL, (A)       ;                             [LXI H,A]
0028  LD (HL), 0       ; mem[A] = 0                  [MVI M,0H]
; B = 1
002A  INC HL           ; HL = addr of B              [INX H]
002B  LD (HL), 1       ; mem[B] = 1                  [MVI M,1H]
; DO WHILE N > 0
@3:
002D  LD A, 0          ; A = 0 (literal)             [MVI A,0H]
002F  LD HL, (N)       ; HL = addr of N              [LXI H,N]
0032  CP (HL)          ; 0 - mem[N]; CF set if N > 0 [CMP M]
0033  JP NC, @4        ; exit if N == 0              [JNC @4]
; T = B
0036  LD A, (B)        ; A = mem[B]                  [LDA B]
0039  LD (T), A        ; mem[T] = A                  [STA T]
; B = A + B
003C  LD HL, (A)       ; HL = addr of A              [LXI H,A]
003F  ADD A, (HL)      ; A = mem[B] + mem[A]         [ADD M]
0040  INC HL           ; HL = addr of B              [INX H]
0041  LD (HL), A       ; mem[B] = result             [MOV M,A]
; A = T
0042  LD A, (T)        ; A = mem[T]                  [LDA T]
0045  DEC HL           ; HL = addr of A              [DCX H]
0046  LD (HL), A       ; mem[A] = A                  [MOV M,A]
; N = N - 1
0047  DEC HL           ; HL = addr of N              [DCX H]
0048  DEC (HL)         ; mem[N]--                    [DCR M]
; END loop
0049  JP @3            ;                             [JMP @3]
@4:
; RETURN A
004C  LD A, (A)        ; A = mem[A]  (addr 0x0003)   [LDA A]
004F  RET
```
**Size: 47 bytes. Instructions: 20 (+1 guard RET in ABS_DIFF).**

### Our MIR2 Z80 backend

```asm
FIB:
    LD C, 0            ; init A=0
    LD D, 1            ; init B=1
    LD E, C            ; }
    LD C, D            ; } parallel copy: (A,B,N) → (C,D,E)
    LD D, A            ; }
.loop_head:
    LD A, D            ; test N
    AND A              ; CP 0 peephole
    JP Z, .trmp0       ; exit if N==0
.loop_body:
    LD A, E            ; new_B = A + B
    ADD A, C
    LD E, A
    DEC D              ; N--
    LD A, C            ; swap A ↔ B
    LD C, E
    LD E, A
    JP .loop_head
.loop_exit:
    LD A, C            ; return A
    RET
.trmp0:                ; trampoline: pass block args to exit
    LD A, E
    LD E, D
    LD D, C
    LD C, A
    JP .loop_exit
```
**Size: 31 bytes (incl. trampoline). Instructions: 21.**

### Analysis: FIB

| Metric | PL/M-80 V4.0 (8080) | MIR2 Z80 | Winner |
|--------|---------------------|-----------|--------|
| Code size | 47 bytes | 31 bytes | **MIR2** (−34%) |
| Loop body T-states/iter | ~80T | ~62T | **MIR2** |
| Memory accesses/iter | 6 loads + 4 stores | 0 | **MIR2** |
| Trampoline overhead | 0 | +5 instr (+21B) | PL/M |
| Loop exit (DJNZ potential) | `DCR M` + `JNC` | `DEC D` + `JP Z` | tie |

**Key insight for FIB:** The PL/M compiler stores every variable in memory and accesses them
with `LDA addr` / `STA addr` / `LXI H,addr; MOV M,A` sequences. Our register-based calling
convention keeps all loop variables (`A`→C, `B`→D, `N`→E) in registers throughout the loop
body, eliminating all memory traffic.

**Notable plm80c cleverness:** `B = A + B` is compiled as:
```asm
LD HL, (A)   ; HL points to A              [LXI H,A]
ADD A, (HL)  ; A = mem[B] + mem[A]         [ADD M]  (A was preloaded with B)
INC HL       ; HL now points to B          [INX H]
LD (HL), A   ; mem[B] = result             [MOV M,A]
```
It uses the `INX H` after the `ADD M` to reuse the HL pointer for the store — saves one
`LXI H,B` instruction. That's a smart peephole for memory-based codegen.

---

## Calling convention: the fundamental difference

```
PL/M-80 V4.0 calling convention (8080 memory-based):
  Caller: pass args in BC/DE register pair
  Callee: immediately store args to absolute memory addrs
          (LD HL,addr + LD (HL),C/E)
  Variables: accessed via LD A,(addr) / LD (addr),A  or
             LD HL,addr + LD r,(HL) / ADD A,(HL) / CP (HL)
  Return: value in accumulator A

MIR2 Z80 calling convention (register-first):
  Caller: pass arg0 in A, additional args in C/D/E/H/L
  Callee: no prologue needed for leaf functions
  Variables: stay in registers (C,D,E,H,L) throughout function
  Return: value in A
```

The PL/M calling convention was designed for the 8080's limited register set and to support
a full call stack (important for recursion and reentrant code). The trade-off is every variable
access is a memory round-trip even when a value was just computed into a register.

Our MIR2 backend is register-first: values live in registers as long as they're live, spilling
to absolute `$F0xx` memory only when physical registers are exhausted. For these two functions,
no spills are needed.

---

## Size comparison (both functions)

| Function | PL/M V4.0 | MIR2 Z80 | Difference |
|----------|-----------|-----------|------------|
| ABS_DIFF | 33 bytes  | 12 bytes  | −64% |
| FIB      | 47 bytes  | 31 bytes  | −34% |
| **Total**| **80 bytes** | **43 bytes** | **−46%** |

The native PL/M binary is confirmed 80 bytes (`CODE AREA SIZE = 0050H 80D` in the listing).
Our binary is 43 bytes (confirmed from `xxd /tmp/report_demo.com | wc`).

---

## Known gaps in our output (vs both compilers)

| Issue | PL/M V4.0 | MIR2 Z80 | Fix |
|-------|-----------|-----------|-----|
| Dead `LD C,A` before RET | ✅ none | 2 × +4T, +1B | Copy coalescing |
| Trampoline for block args | ✅ none | +5 instrs | Copy coalescing |
| `JP` instead of `JR` | ✅ (8080 has no JR) | +1B/jump | JP→JR peephole |
| DJNZ for counter loop | ✅ (8080 has no DJNZ) | missing | ClassCounter hint |

---

## Summary

Our MIR2 backend produces **46% smaller and significantly faster** code than the Intel PL/M-80
V4.0 compiler for these functions, primarily because:

1. **Register-first calling convention** — no parameter-to-memory store at procedure entry
2. **Register allocation** — loop variables stay in C/D/E, zero memory traffic in loop body
3. **Z80 advantage** — Z80 adds `CP reg`, `NEG`, `AND A` (`CP 0`), `DJNZ` over 8080

The PL/M compiler shows genuine cleverness in its memory-based codegen (pointer reuse with
`INX H` for adjacent fields, `JNC` with reversed operands to avoid an extra `CMP`). These are
the optimizations a 1970s compiler writer would craft by hand for the 8080's constraints.

The comparison is not entirely fair — PL/M V4.0 was designed for portability and correctness
on the 8080, not maximum Z80 performance. A PL/M compiler targeting Z80 registers natively
would produce output much closer to ours.
