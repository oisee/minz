# Z80 Complete Operation Reference

> Local cheat sheet — all instructions, T-states, flags, encoding notes.
> Authoritative for this project. Covers full Z80 including IX/IY halves (production-safe).
> Flag columns: S Z H P/V N C  |  `*`=affected  `–`=unchanged  `0`=reset  `1`=set  `V`=overflow  `P`=parity

---

## 8-Bit Loads

### Register ↔ Register

| Instruction | T | Flags | Notes |
|-------------|---|-------|-------|
| `LD r, r'` | 4 | – – – – – – | r,r' ∈ {A,B,C,D,E,H,L}. 7×7=49 combos (LD H,L etc.). `LD H,H` = NOP equivalent |
| `LD r, IXH/IXL/IYH/IYL` | 8 | – – – – – – | r ∈ {A,B,C,D,E} only. H,L CANNOT be dst (DD/FD prefix hijacks H/L encoding) |
| `LD IXH/IXL/IYH/IYL, r` | 8 | – – – – – – | r ∈ {A,B,C,D,E,IXH,IXL} only (IX group) or {A,B,C,D,E,IYH,IYL} (IY group) |
| `LD IXH, IXL` / `LD IXL, IXH` | 8 | – – – – – – | Cross within IX pair OK |
| `LD IYH, IYL` / `LD IYL, IYH` | 8 | – – – – – – | Cross within IY pair OK |
| `LD H, IXH` | ✗ | — | IMPOSSIBLE — DD prefix hijacks H operand encoding |

### Register ← Immediate

| Instruction | T | Flags | Notes |
|-------------|---|-------|-------|
| `LD r, n` | 7 | – – – – – – | r ∈ {A,B,C,D,E,H,L} |
| `LD IXH, n` / `LD IXL, n` | 11 | – – – – – – | DD 26 n / DD 2E n |
| `LD IYH, n` / `LD IYL, n` | 11 | – – – – – – | FD 26 n / FD 2E n |
| `LD (HL), n` | 10 | – – – – – – | Write immediate to address in HL |

### Register ↔ Memory

| Instruction | T | Flags | Notes |
|-------------|---|-------|-------|
| `LD A, (BC)` | 7 | – – – – – – | A ← mem[BC] |
| `LD A, (DE)` | 7 | – – – – – – | A ← mem[DE] |
| `LD A, (nn)` | 13 | – – – – – – | A ← mem[nn]. 3-byte instruction |
| `LD (BC), A` | 7 | – – – – – – | mem[BC] ← A |
| `LD (DE), A` | 7 | – – – – – – | mem[DE] ← A |
| `LD (nn), A` | 13 | – – – – – – | mem[nn] ← A. 3-byte instruction |
| `LD r, (HL)` | 7 | – – – – – – | r ∈ {A,B,C,D,E,H,L} ← mem[HL] |
| `LD (HL), r` | 7 | – – – – – – | mem[HL] ← r |
| `LD r, (IX+d)` | 19 | – – – – – – | r ∈ {A,B,C,D,E,H,L}; d = signed offset byte |
| `LD (IX+d), r` | 19 | – – – – – – | mem[IX+d] ← r |
| `LD r, (IY+d)` | 19 | – – – – – – | Same with IY |
| `LD (IY+d), r` | 19 | – – – – – – | |
| `LD (IX+d), n` | 19 | – – – – – – | Write immediate to IX-indexed address |
| `LD (IY+d), n` | 19 | – – – – – – | |

### Special Register Loads (I, R)

| Instruction | T | Flags | Notes |
|-------------|---|-------|-------|
| `LD I, A` | 9 | – – – – – – | ED 47. I ← A. Interrupt vector high byte |
| `LD R, A` | 9 | – – – – – – | ED 4F. R ← A. **Bit 7 preserved; bits 0-6 auto-increment per M1 cycle** |
| `LD A, I` | 9 | * * 0 * 0 – | ED 57. A ← I. S,Z set from I; P/V = IFF2 (interrupt enable state!) |
| `LD A, R` | 9 | * * 0 * 0 – | ED 5F. A ← R. S,Z set from R; P/V = IFF2; **R bits 0-6 are refresh counter garbage** |

> **R-register boolean spill**: store 0x00/0xFF → `LD R,A` (9T). Restore: `LD A,R; ADD A,A; SBC A,A` (17T). 26T round trip, 0 named registers consumed. Bit 7 is the payload.

---

## 16-Bit Loads

### Register ← Immediate

| Instruction | T | Flags | Notes |
|-------------|---|-------|-------|
| `LD BC, nn` | 10 | – – – – – – | |
| `LD DE, nn` | 10 | – – – – – – | |
| `LD HL, nn` | 10 | – – – – – – | |
| `LD SP, nn` | 10 | – – – – – – | |
| `LD IX, nn` | 14 | – – – – – – | DD 21 lo hi |
| `LD IY, nn` | 14 | – – – – – – | FD 21 lo hi |

### Register ↔ Memory (nn = 16-bit address)

| Instruction | T | Flags | Notes |
|-------------|---|-------|-------|
| `LD HL, (nn)` | 16 | – – – – – – | **Cheapest 16-bit memory load** (no ED prefix) |
| `LD (nn), HL` | 16 | – – – – – – | **Cheapest 16-bit memory store** |
| `LD BC, (nn)` | 20 | – – – – – – | ED 4B lo hi |
| `LD DE, (nn)` | 20 | – – – – – – | ED 5B lo hi |
| `LD SP, (nn)` | 20 | – – – – – – | ED 7B lo hi — useful to save/restore SP |
| `LD (nn), BC` | 20 | – – – – – – | ED 43 lo hi |
| `LD (nn), DE` | 20 | – – – – – – | ED 53 lo hi |
| `LD (nn), SP` | 20 | – – – – – – | ED 73 lo hi |
| `LD IX, (nn)` | 20 | – – – – – – | DD 2A lo hi |
| `LD (nn), IX` | 20 | – – – – – – | DD 22 lo hi |
| `LD IY, (nn)` | 20 | – – – – – – | FD 2A lo hi |
| `LD (nn), IY` | 20 | – – – – – – | FD 22 lo hi |

### Inter-Register

| Instruction | T | Flags | Notes |
|-------------|---|-------|-------|
| `LD SP, HL` | 6 | – – – – – – | Fast SP ← HL |
| `LD SP, IX` | 10 | – – – – – – | DD F9 |
| `LD SP, IY` | 10 | – – – – – – | FD F9 |

### Stack

| Instruction | T | Flags | Notes |
|-------------|---|-------|-------|
| `PUSH BC/DE/HL/AF` | 11 | – – – – – – | SP -= 2, mem[SP] ← rr |
| `PUSH IX` / `PUSH IY` | 15 | – – – – – – | DD E5 / FD E5 |
| `POP BC/DE/HL/AF` | 10 | – – – – – – | rr ← mem[SP], SP += 2 |
| `POP IX` / `POP IY` | 14 | – – – – – – | DD E1 / FD E1 |

> **PUSH/POP idiom** — move any 16-bit pair to any other: `PUSH BC; POP DE` = 21T, no flags. Useful cross-pair transfer when direct LD impossible.

---

## Exchange

| Instruction | T | Flags | Notes |
|-------------|---|-------|-------|
| `EX DE, HL` | 4 | – – – – – – | Swap DE↔HL. Critical for DEHL u32 ops |
| `EX AF, AF'` | 4 | – – – – – – | Swap AF↔AF'. A' and F' survive. **EX AF,AF' bridges**: BC,DE,HL,IXH..IYL all survive |
| `EXX` | 4 | – – – – – – | Swap BC↔BC', DE↔DE', HL↔HL' simultaneously. **EXX bridges**: A,IXH,IXL,IYH,IYL survive |
| `EX (SP), HL` | 19 | – – – – – – | Swap HL with top of stack. Slow but no register used |
| `EX (SP), IX` | 23 | – – – – – – | DD E3 |
| `EX (SP), IY` | 23 | – – – – – – | FD E3 |

> **Universal bridges** (survive BOTH EXX and EX AF,AF'): IXH, IXL, IYH, IYL — zero zone-crossing cost.

---

## 8-Bit Arithmetic

### ADD / ADC (dst always A)

| Instruction | T | S Z H V N C | Notes |
|-------------|---|-------------|-------|
| `ADD A, r` | 4 | * * * V 0 * | r ∈ {A,B,C,D,E,H,L} |
| `ADD A, IXH/IXL/IYH/IYL` | 8 | * * * V 0 * | IX halves as src — works! |
| `ADD A, (HL)` | 7 | * * * V 0 * | |
| `ADD A, (IX+d)` | 19 | * * * V 0 * | |
| `ADD A, n` | 7 | * * * V 0 * | |
| `ADC A, r` | 4 | * * * V 0 * | r ∈ {A,B,C,D,E,H,L} only — **NO IX half variant** |
| `ADC A, (HL)` | 7 | * * * V 0 * | |
| `ADC A, n` | 7 | * * * V 0 * | |

### SUB / SBC

| Instruction | T | S Z H V N C | Notes |
|-------------|---|-------------|-------|
| `SUB r` | 4 | * * * V 1 * | r ∈ {A,B,C,D,E,H,L} |
| `SUB IXH/IXL/IYH/IYL` | 8 | * * * V 1 * | IX halves as src — works! |
| `SUB n` | 7 | * * * V 1 * | |
| `SBC A, r` | 4 | * * * V 1 * | r ∈ {A,B,C,D,E,H,L} only — **NO IX half variant** |
| `SBC A, n` | 7 | * * * V 1 * | |
| **`SBC A, A`** | **4** | **\* \* \* \* 1 0** | **→ 0xFF if CY=1, 0x00 if CY=0. Universal boolean materializer** |

### AND / OR / XOR / CP

| Instruction | T | S Z H P N C | Notes |
|-------------|---|-------------|-------|
| `AND r` | 4 | * * 1 P 0 0 | r ∈ {A,B,C,D,E,H,L} |
| `AND IXH/IXL` | 8 | * * 1 P 0 0 | IX halves — works! |
| `AND n` | 7 | * * 1 P 0 0 | |
| `OR r` | 4 | * * 0 P 0 0 | |
| `OR IXH/IXL` | 8 | * * 0 P 0 0 | |
| `OR n` | 7 | * * 0 P 0 0 | |
| `XOR r` | 4 | * * 0 P 0 0 | `XOR A` → A=0, CY=0, Z=1 (4T zero) |
| `XOR IXH/IXL` | 8 | * * 0 P 0 0 | |
| `XOR n` | 7 | * * 0 P 0 0 | |
| `CP r` | 4 | * * * V 1 * | Compare A with r, set flags, discard result |
| `CP IXH/IXL` | 8 | * * * V 1 * | |
| `CP n` | 7 | * * * V 1 * | |

### INC / DEC (8-bit)

| Instruction | T | S Z H V N C | Notes |
|-------------|---|-------------|-------|
| `INC r` | 4 | * * * V 0 – | r ∈ {A,B,C,D,E,H,L}. **CY unchanged!** |
| `INC IXH/IXL/IYH/IYL` | 8 | * * * V 0 – | IX/IY halves — works! |
| `INC (HL)` | 11 | * * * V 0 – | |
| `INC (IX+d)` | 23 | * * * V 0 – | |
| `DEC r` | 4 | * * * V 1 – | CY unchanged |
| `DEC IXH/IXL/IYH/IYL` | 8 | * * * V 1 – | |
| `DEC (HL)` | 11 | * * * V 1 – | |
| `DEC (IX+d)` | 23 | * * * V 1 – | |

### Unary (A only)

| Instruction | T | Flags | Notes |
|-------------|---|-------|-------|
| `NEG` | 8 | * * * V 1 * | ED 44. A = 0 - A (two's complement). Only A. |
| `CPL` | 4 | – – 1 – 1 – | A = ~A (one's complement). **Free boolean NOT for 0xFF/0x00** |
| `DAA` | 4 | * * * P – * | BCD adjust after ADD/SUB |
| `SCF` | 4 | – – 0 – 0 1 | CY = 1 |
| `CCF` | 4 | – – * – 0 * | CY = ~CY |
| `OR A` | 4 | * * 0 P 0 0 | Clears CY, sets S/Z/P from A. Common "clear carry" idiom |

---

## 16-Bit Arithmetic

### ADD HL / ADC HL / SBC HL

| Instruction | T | S Z H V N C | Notes |
|-------------|---|-------------|-------|
| `ADD HL, BC/DE/HL/SP` | 11 | – – * – 0 * | CY set, S/Z/V unchanged (!) |
| `ADC HL, BC/DE/HL/SP` | 15 | * * * V 0 * | ED prefix. Full flags |
| `SBC HL, BC/DE/HL/SP` | 15 | * * * V 1 * | ED prefix. Full flags |
| `ADD IX, BC/DE/IX/SP` | 15 | – – * – 0 * | DD prefix. IX as accumulator |
| `ADD IY, BC/DE/IY/SP` | 15 | – – * – 0 * | FD prefix. IY as accumulator |

> **Note**: ADD HL only updates CY and H, not S/Z/V. ADC/SBC HL update all flags. This is why HLH'L' 32-bit add uses ADC HL for the high half — we need the carry propagation.

### INC / DEC (16-bit)

| Instruction | T | Flags | Notes |
|-------------|---|-------|-------|
| `INC BC/DE/HL/SP` | 6 | – – – – – – | **NO flag changes at all!** |
| `DEC BC/DE/HL/SP` | 6 | – – – – – – | **NO flag changes at all!** |
| `INC IX` / `INC IY` | 10 | – – – – – – | |
| `DEC IX` / `DEC IY` | 10 | – – – – – – | |

---

## Rotate and Shift

### Fast accumulator rotates (4T, no CB prefix)

| Instruction | T | S Z H P N C | Notes |
|-------------|---|-------------|-------|
| `RLCA` | 4 | – – 0 – 0 * | A rotate left, bit 7 → CY and bit 0. No S/Z/P change |
| `RRCA` | 4 | – – 0 – 0 * | A rotate right, bit 0 → CY and bit 7 |
| `RLA` | 4 | – – 0 – 0 * | A rotate left through CY (9-bit rotate) |
| `RRA` | 4 | – – 0 – 0 * | A rotate right through CY |

> `RLCA` / `RLA` are the cheapest way to extract bit 7 into CY (for boolean recovery from 0xFF/0x00).

### CB-prefix rotates/shifts (all registers, 8T; (HL) = 15T)

| Instruction | T | Flags | Notes |
|-------------|---|-------|-------|
| `RLC r` | 8 | * * 0 P 0 * | Rotate left, bit7→CY and bit0 |
| `RRC r` | 8 | * * 0 P 0 * | Rotate right, bit0→CY and bit7 |
| `RL r` | 8 | * * 0 P 0 * | Rotate left through CY |
| `RR r` | 8 | * * 0 P 0 * | Rotate right through CY |
| `SLA r` | 8 | * * 0 P 0 * | Shift left arithmetic (bit7→CY, bit0=0) |
| `SRA r` | 8 | * * 0 P 0 * | Shift right arithmetic (sign extend, bit0→CY) |
| `SRL r` | 8 | * * 0 P 0 * | Shift right logical (bit7=0, bit0→CY) |
| `SLL r` | 8 | * * 0 P 0 * | Shift left "logical" (bit7→CY, **bit0=1**) — undocumented but useful |

> r ∈ {A,B,C,D,E,H,L}. For (HL): 15T. For (IX+d): 23T (DDCB prefix).
> `SRL A` = 8T; `RRCA` = 4T (prefer RRCA for A). `SLA A` = 8T; `ADD A,A` / `RLCA` = 4T (prefer these for A).

### Digit rotate (BCD)

| Instruction | T | Flags | Notes |
|-------------|---|-------|-------|
| `RLD` | 18 | * * 0 P 0 – | ED 6F. 4-bit rotate: A[3:0] → (HL)[7:4], (HL)[7:4] → (HL)[3:0], (HL)[3:0] → A[3:0] |
| `RRD` | 18 | * * 0 P 0 – | ED 67. Reverse direction |

---

## Bit Manipulation (CB prefix)

| Instruction | T | Flags | Notes |
|-------------|---|-------|-------|
| `BIT b, r` | 8 | – * 1 * 0 – | Z = ~bit b of r. Does NOT modify r |
| `BIT b, (HL)` | 12 | – * 1 * 0 – | |
| `BIT b, (IX+d)` | 20 | – * 1 * 0 – | DDCB prefix |
| `SET b, r` | 8 | – – – – – – | Set bit b in r |
| `SET b, (HL)` | 15 | – – – – – – | |
| `SET b, (IX+d)` | 23 | – – – – – – | |
| `RES b, r` | 8 | – – – – – – | Clear bit b in r |
| `RES b, (HL)` | 15 | – – – – – – | |
| `RES b, (IX+d)` | 23 | – – – – – – | |

> b ∈ {0..7}. r ∈ {A,B,C,D,E,H,L}.

---

## Control Flow

### Jumps (absolute)

| Instruction | T | Notes |
|-------------|---|-------|
| `JP nn` | 10 | Unconditional |
| `JP NZ/Z/NC/C/PO/PE/P/M, nn` | 10 | Conditional (always 10T — no branch penalty!) |
| `JP (HL)` | 4 | Jump to address in HL. Fastest indirect jump |
| `JP (IX)` / `JP (IY)` | 8 | |

### Jumps (relative, ±127 bytes)

| Instruction | T | Notes |
|-------------|---|-------|
| `JR e` | 12 | Unconditional relative |
| `JR NZ/Z/NC/C, e` | 12 / 7 | 12T if taken, 7T if not taken |
| `DJNZ e` | 13 / 8 | B-- then jump if B≠0. 13T taken, 8T not taken |

> **Branch penalty** (relative): 5T extra if taken vs not-taken. Branchless sequences typically need 15-24T overhead to beat branching — so **branch > branchless on Z80** for most cases.

### Call / Return

| Instruction | T | Notes |
|-------------|---|-------|
| `CALL nn` | 17 | Push PC, jump |
| `CALL NZ/Z/NC/C/PO/PE/P/M, nn` | 17 / 10 | 17T if taken, 10T if not |
| `RET` | 10 | Pop PC |
| `RET NZ/Z/NC/C/PO/PE/P/M` | 11 / 5 | 11T if taken, 5T if not |
| `RETI` | 14 | ED 4D. Return from interrupt (signals IFF1 restore to daisy chain) |
| `RETN` | 14 | ED 45. Return from NMI, restores IFF1 from IFF2 |
| `RST p` | 11 | Call to fixed addresses 00,08,10,18,20,28,30,38h |

---

## Block Operations

### Load block

| Instruction | T | Flags | Notes |
|-------------|---|-------|-------|
| `LDI` | 16 | – – 0 * 0 – | mem[DE]←mem[HL], HL++, DE++, BC--. P/V=1 if BC≠0 after |
| `LDIR` | 21/16 | – – 0 0 0 – | Repeat LDI until BC=0. 21T per byte (loop), 16T last |
| `LDD` | 16 | – – 0 * 0 – | Like LDI but HL--, DE-- |
| `LDDR` | 21/16 | – – 0 0 0 – | Repeat LDD until BC=0 |

### Compare block

| Instruction | T | Flags | Notes |
|-------------|---|-------|-------|
| `CPI` | 16 | * * * * 1 – | Compare A with mem[HL], HL++, BC--. Z=1 if match |
| `CPIR` | 21/16 | * * * * 1 – | Repeat until match or BC=0 |
| `CPD` / `CPDR` | 16/21 | * * * * 1 – | Decrementing variants |

### I/O block

| Instruction | T | Flags | Notes |
|-------------|---|-------|-------|
| `INI` / `INIR` | 16/21 | * * * * 1 * | IN (C)→mem[HL], HL++, B-- |
| `IND` / `INDR` | 16/21 | | Decrementing |
| `OUTI` / `OTIR` | 16/21 | * * * * 1 * | OUT (C)←mem[HL], HL++, B-- |
| `OUTD` / `OTDR` | 16/21 | | Decrementing |

---

## I/O

| Instruction | T | Flags | Notes |
|-------------|---|-------|-------|
| `IN A, (n)` | 11 | – – – – – – | A ← port[n]. Port addr = (A<<8)\|n |
| `IN r, (C)` | 12 | * * 0 P 0 – | r ← port[C]. Port addr = BC. Sets flags! |
| `IN F, (C)` | 12 | * * 0 P 0 – | ED 70. Read port, discard data, keep flags (undocumented) |
| `OUT (n), A` | 11 | – – – – – – | port[n] ← A |
| `OUT (C), r` | 12 | – – – – – – | port[C] ← r. r ∈ {A,B,C,D,E,H,L} |
| `OUT (C), 0` | 12 | – – – – – – | ED 71. Output 0 (undocumented). Useful for port manipulation |

---

## Interrupt Control

| Instruction | T | Flags | Notes |
|-------------|---|-------|-------|
| `DI` | 4 | – – – – – – | Disable maskable interrupts (IFF1=IFF2=0) |
| `EI` | 4 | – – – – – – | Enable maskable interrupts. Effect delayed one instruction |
| `IM 0` | 8 | – – – – – – | Interrupt mode 0: device puts instruction on bus |
| `IM 1` | 8 | – – – – – – | Interrupt mode 1: RST 38h. **ZX Spectrum default** |
| `IM 2` | 8 | – – – – – – | Interrupt mode 2: I:R vector table lookup (I=high byte) |
| `HALT` | 4 | – – – – – – | CPU halts, executes NOP until interrupt |
| `NOP` | 4 | – – – – – – | No operation |

---

## Spill / Channel Tier Hierarchy

Complete register pressure escape hatch hierarchy:

| Tier | Method | Cost | Capacity | Safe | Constraint |
|------|--------|------|----------|------|-----------|
| **L0** | Primary regs A..L | **0T** | 7 bytes | always | — |
| **L1** | IXH IXL IYH IYL | **8T** access | 4 bytes | always | H/L↔IX = 16T; no ADC/SBC src |
| **L1.5** | R register | **26T** round trip | **1 bit** (bit 7) | IM 0/1 | Store 0x00/0xFF; recover with `ADD A,A; SBC A,A` |
| **L2a** | I register | **18T** round trip | 8 bits | IM 0/1 | `LD I,A` / `LD A,I`. Flags clobbered on read |
| **L2b** | TSMC tunnel | **20T** (8-bit) | 8 bits | DI/EI or non-recursive | Self-modifying `LD (tsmc+1),A` |
| **L3** | PUSH/POP (SP) | **21T** (16-bit) | 16 bits | always | SP is zone-invariant; recursion-safe |
| **L4a** | Memory (nn) 8-bit | **13T** load/store | 8 bits | always | Only A: `LD A,(nn)` / `LD (nn),A` |
| **L4b** | Memory (nn) HL | **16T** load/store | 16 bits | always | Cheapest pair: `LD HL,(nn)` / `LD (nn),HL` |
| **L4c** | Memory (nn) BC/DE/SP/IX/IY | **20T** load/store | 16 bits | always | ED/DD/FD prefix |

> **Zone channels** (cross EXX / EX AF,AF' boundaries without cost):
> - 0T: IX/IY halves (universal — survive BOTH swaps)
> - 0T: A (survives EXX); BC/DE/HL (survive EX AF,AF')
> - 18T: I register (8-bit, zone-invariant)
> - 18T/26T: R register (1-bit boolean)
> - 20T: TSMC tunnel (code memory is zone-invariant)
> - 21T: PUSH/POP (SP is zone-invariant, never swapped)

---

## Boolean Representation: 0x00/0xFF (canonical)

**Adopted standard for this project.** Proven optimal by exhaustive search.

| Operation | 0x00/0xFF cost | 0x00/0x01 cost | Savings |
|-----------|---------------|----------------|---------|
| NOT | `CPL` 4T | `XOR 1` 7T | 3T |
| AND | `AND r` 4T | `AND r; AND 1` 11T | 7T |
| OR | `OR r` 4T | `OR r; AND 1` 11T | 7T |
| CY → bool | `SBC A,A` 4T | `LD A,0; ADC A,0` 11T | 7T |
| bool → CY | `ADD A,A` 4T | `CP 1` or `RRCA` varies | — |
| CMOV | `SBC A,A; AND x; XOR y` | shift/rotate sequence | — |

> `SBC A,A` is the **universal materializer**: CY=1 → A=0xFF, CY=0 → A=0x00.
> `CPL` is the **universal boolean NOT**: 0xFF↔0x00 in 4T.

---

*Last updated: 2026-04-01. For GPU optimizer architecture see CLAUDE.md.*
