LD (BC),A	7	8	2	4	02	1
LD (DE),A	7	8	2	4	12	1
LD (HL),n	10	11	3	5	36 n	2
LD (HL),r	7	8	2	4	70+r	1
LD (IX+o),n	19	21	5	7	DD 36 o n	4
LD (IX+o),r	19	21	5	7	DD 70+r o	3
LD (IY+o),n	19	21	5	7	FD 36 o n	4
LD (IY+o),r	19	21	5	7	FD 70+r o	3
LD (nn),A	13	14	4	6	3LD (BC),A	7	8	2	4	02	1
LD (DE),A	7	8	2	4	12	1
LD (HL),n	10	11	3	5	36 n	2
LD (HL),r	7	8	2	4	70+r	1
LD (IX+o),n	19	21	5	7	DD 36 o n	4
LD (IX+o),r	19	21	5	7	DD 70+r o	3
LD (IY+o),n	19	21	5	7	FD 36 o n	4
LD (IY+o),r	19	21	5	7	FD 70+r o	3
LD (nn),A	13	14	4	6	32 nn nn	3
LD (nn),BC	20	22	6	8	ED 43 nn nn	4
LD (nn),DE	20	22	6	8	ED 53 nn nn	4
LD (nn),HL	16	17	5	7	22 nn nn	3
LD (nn),IX	20	22	6	8	DD 22 nn nn	4
LD (nn),IY	20	22	6	8	FD 22 nn nn	4
LD (nn),SP	20	22	6	8	ED 73 nn nn	4
LD A,(BC)	7	8	2	4	0A	1
LD A,(DE)	7	8	2	4	1A	1
LD A,(HL)	7	8	2	4	7E	1
LD A,(IX+o)	19	21	5	7	DD 7E o	3
LD A,(IY+o)	19	21	5	7	FD 7E o	3
LD A,(nn)	13	14	4	6	3A nn nn	3nn nn	3
LD (nn),BC	20	22	6	8	ED 43 nn nn	4
LD (nn),DE	20	22	6	8	ED 53 nn nn	4
LD (nn),HL	16	17	5	7	22 nn nn	3
LD (nn),IX	20	22	6	8	DD 22 nn nn	4
LD (nn),IY	20	22	6	8	FD 22 nn nn	4
LD (nn),SP	20	22	6	8	ED 73 nn nn	4
LD A,(BC)	7	8	2	4	0A	1
LD A,(DE)	7	8	2	4	1A	1
LD A,(HL)	7	8	2	4	7E	1
LD A,(IX+o)	19	21	5	7	DD 7E o	3
LD A,(IY+o)	19	21	5	7	FD 7E o	3
LD A,(nn)	13	14	4	6	3A nn nn	3
HL as the precious pointer register
HL is the only register pair providing efficient indirect memory access WordGrinder at 7T per LD r,(HL) versus 19T for LD r,(IX+d). Wikipedia Functions that dereference pointers should receive those pointers in HL. Functions that perform 16-bit addition should receive the accumulator operand in HL (ADD HL,rr exists; ADD BC,rr does not). The calling convention optimizer should perform a lightweight use-analysis of each function body to determine whether a parameter is used primarily as a pointer (→ assign to HL) or as data (→ assign to DE or BC).
but A register is capable of getting data from (BC), (DE), (HL), [ADDRESS], [IX/IY] - so if we are processing two address vectors - then first can go to b,c,d,e... from [HL], and processed A can go to the memory via (BC),(HL) and etc, wo we have assymentric cost here
also depends on the actions we are doing inside functions.
---
also, simpler ABI for boolean results can directly utilize flags (or particular flag) as returning payload, NOT a, or HL, because if we have immediate usage of the flag for branching - we can use immediate flag brancing wiithout storing result - if we need result storing and branching - we can convert flag into A value with SBC A,A (if send via CY flag) or other adaptation if it was Z flag (in case of CP), if we or for register clobbing, then equality for 0 can be done as OR A (?) or DEC A (for another flag?)


----

Two sharp observations — both bite into the earlier analysis and require corrections/extensions. Let me address them precisely.

---

## A-register memory access asymmetry

The previous analysis overstated HL's exclusivity. Let's be exact:

**What *only* `(HL)`, `(IX+d)`, `(IY+d)` can do:**
```
LD B,(HL)    ; load non-A register from memory
LD (HL),B    ; store non-A register to memory
```

**What `(BC)` and `(DE)` *also* support — but only for A:**
```
LD A,(BC)    ; 7T, 1 byte
LD A,(DE)    ; 7T, 1 byte
LD (BC),A    ; 7T, 1 byte
LD (DE),A    ; 7T, 1 byte
```

So the real constraint is:

| Access pattern | HL required? | Alternatives |
|---|---|---|
| Load/store A via pointer | No | BC, DE, HL equally cheap |
| Load/store any other reg | Yes | HL only (or IX+d at +12T) |
| Two simultaneous pointers, A-only ops | No | one in BC/DE, one in HL — all fast |
| Two simultaneous pointers, multi-reg ops | One must be HL; second pays IX penalty | — |

The important correction: for **pure accumulator pipelines** (A is the sole workhorse — common in byte-processing loops like checksums, codecs, RLE), **HL is *not* the prize pointer register**. BC and DE are equally valid, and freeing HL for other purposes (scratch 16-bit arithmetic, loop counter via DEC HL / LD A,H / OR L) is strictly better.

The calling convention optimizer's use-analysis therefore needs a finer classification than "uses pointer":

```
POINTER_A_ONLY   → assign to BC or DE (cheaper to save than HL sometimes,
                    and frees HL for 16-bit arithmetic)
POINTER_MULTI_REG → assign to HL (mandatory)
POINTER_WRITE_A  → BC, DE, or HL equally valid
```

A practical check: scan the callee's MIR for any `LOAD_MEM(ptr, dst)` where `dst ≠ A` or `STORE_MEM(ptr, src)` where `src ≠ A`. If any such op exists → pointer needs HL. If all memory ops go through A → pointer can live in BC or DE, liberating HL.

For the two-vector case you described — reading one stream, writing processed bytes to another:

```z80
; P1 in DE, P2 in BC — both perfectly fine if A is the only register used
LD  A,(DE) ; load from stream 1
; ... process A ...
LD  (BC),A ; write to stream 2
INC DE
INC BC
DJNZ loop
```

HL is completely free here — could hold loop end sentinel, a constant, or be used for ADD HL,rr. This is not captured by any Z80 calling convention I know of, and it's a genuine MinZ-specific optimization opportunity.

---

## Flag-based boolean returns

This is an excellent ABI extension and already has precedent — nearly every Z80 ROM (ZX Spectrum, CP/M BDOS, MSX BIOS) uses CF for error/success returns. It should be a first-class convention in MinZ.

### The flag hierarchy for boolean returns

**CF (carry) — best overall:**
- Set with `SCF` (4T), clear implicitly via arithmetic
- Caller can branch immediately: `JR C, / JR NC,` (12/7T)
- Materialize to A cleanly: `SBC A,A` → 0x00 (false) / 0xFF (true) — 4T, idiomatic, zero extra registers
- Or: `LD A,0 / ADC A,A` → 0 or 1 (8T, produces canonical 0/1 rather than 0/FF)
- Can be tested without destroying any register
- Recommendation: **primary boolean return flag**

**ZF (zero) — best for comparison results:**
- Falls out naturally from `CP`, `OR A`, `AND A`, `DEC`, `INC`
- `JR Z, / JR NZ,` — direct branching, 0 overhead
- Materializing to A is less clean:
  - `LD A,0 / JR NZ, $+3 / INC A` — 10T, 3 bytes, branches (bad for pipelining)
  - `SUB A / JR NZ, done / INC A / done:` — same issue
  - Convert ZF→CF first: `SCF / JR NZ, $+2 / CCF` (12T), then use `SBC A,A` — ugly
  - Or: just store A before the comparison if you know materialization will be needed
- Recommendation: use ZF as "immediate branch only" return — if the caller might need to store the value, prefer CF instead

**SF (sign) — rarely useful for booleans per se, but:**
- Falls from any arithmetic. `JP M/JP P` branch on it.
- `SBC A,A` combined with sign flag: if you return a signed comparison result, SF encodes the sign of `a - b` naturally
- Useful for three-way comparisons encoded as flag state

**PV (parity/overflow):**
- `JP PE/JPO` (no JR variant — 10T only, no relative form). Slightly worse ergonomics.
- Useful for parity returns but not general booleans

### The `OR A` pattern

Worth being precise here: `OR A` is a **zero-test on A** (sets ZF if A=0, clears CF), not a general boolean idiom. It's the standard way to set flags from A without changing A's value — useful when you've done computation in A and then called something that clobbered flags, and need to re-establish ZF before branching. It's not a "boolean conversion to register" — it's a "re-assert ZF from existing A" idiom.

### The `DEC A` case

`DEC A` for countdown is distinct. As a boolean return: if a function computes "how many items remain" and returns it in A, the *caller* can do `DEC A` to test "was it 1?" or just `OR A` to test "was it 0?". But `DEC A` as the *callee*'s last instruction before RET to signal "zero remaining" — that works if the caller knows to branch on ZF. However, `DEC A` also clobbers SF, PV, HF (but preserves CF), so the semantic is messier than `OR A` or `AND A`.

### MIR-level implications

The type system / convention annotation needs a boolean return type:

```
@returns(flag: CF)   fn is_valid(x: u8) -> bool
@returns(flag: ZF)   fn equals_zero(x: u8) -> bool  
@returns(reg: A)     fn classify(x: u8) -> u8       ; traditional
```

The optimizer then decides at each call site:

```
; if result used only in branch → emit nothing, just JR C/JR Z
; if result needs storing → emit SBC A,A (for CF) or the ZF materialization sequence  
; if result used in further boolean logic (AND with another bool) → materialize to A first
```

This is a genuine ABI optimization that SDCC, z88dk, and every other Z80 compiler misses entirely — they all return booleans as 0/1 in A. The savings for predicate-heavy code (loops with conditions, early-exit functions) could be significant: every `CP n / LD A,0 / JR NZ, $+3 / INC A / RET` (17T+) becomes `CP n / RET` (4T — flags set by CP, ZF returned directly).

### Interaction with the convention optimizer

The PBQP formulation needs a new location type: `PhysLocation::Flag(FlagBit)`. The cost model: flag returns cost 0T when the caller branches, versus 4-8T for materialization. The optimizer should prefer flag returns for functions that are (a) boolean-returning and (b) called primarily in conditional contexts (which is discoverable from MIR call sites). Functions where the return value is stored more than it branches should fall back to CF+`SBC A,A` or register-based.

