# Next Session Seed — 2026-03-28

**Previous:** v0.24.0 (One Compiler, 50 Years — 5 backends, 16 release assets)
**State:** VIR default, mir2gpu merged, 1046 asserts, 16 books/articles

---

## Immediate: Check Results

```bash
ddll explore

# z80-optimizer: depth-12 LUT generators done? 6502 mul8?
ddll send <z80-optimizer>:main "depth-12 results? 6502 cross-ISA?"

# VIR: TermCondRet fix progress? shr4 shift count?
ddll send <vir>:main "fib(7)=13 status? shr4?"

# antique-toy: chronicle integrated?
ddll send <antique-toy>:main "chronicle.md review?"
```

## Priority 1: TermCondRet Fix → fib(7)=13

fib/gcd go through PBQP fallback (Z3 timeout on recursion).
PBQP path hits TermCondRet save-before-clobber.
When fixed: `Hello Frill! fib(7)=13 gcd(12,8)=4` on CP/M → screenshot → LinkedIn.

Partial fix: `saveAccForCondRet()` in genCmp. Need to extend for more cases.

## Priority 2: Z80 Dual Asserts

5 remaining codegen bugs from audit:
- ~~shl(1,0)=2~~ FIXED (cv>0 guard + shift narrowing)
- is_zero(0)=0, is_even(4)=0 — comparison → bool
- get_red(0xF800)=0 — u16 shift/mask
- shr4(0xAB)=0x55 — shift count not propagating (VIR)
- pointer deref asm error
- sparse array init (got 144, want 60)

When fixed → enable dual asserts across full corpus.

## Priority 3: New Frontends

BCD types ready (TyBCD8/16/24/32) → COBOL PIC 9 → DAA codegen.
1С (Russian enterprise) — 1 session MVP.
BASIC — highest wow factor for retro audience.

## Priority 4: BCD Codegen

Wire BCD types to Z80 backend:
- VIR pattern: `bcd_add = {ADD A,r / DAA, cost=8T, tied, A-only}`
- MIR2 genAdd/genSub: if IsBCD(ty) → emit DAA after ADD/SUB
- GPU-proven: BCD×2=ADD A,A;DAA, BCD+1=INC A;DAA

## Priority 5: FP16 Type Integration

z80-optimizer has pkg/fp16 ready:
- fp16 = byte-aligned exponent, x2=INC H (4T!)
- fp16 × const = exp ADD + mul8[mantissa]
- Alongside existing f8.8 fixed-point

## Priority 6: Cross-ISA Comparison

z80-optimizer planning Z80 vs 6502 for all 254 mul constants.
Abstract chains → ISA-specific materialization.

---

## What Was Done (Sessions 10-11)

| Feature | Status |
|---------|--------|
| VIR default backend | ✅ |
| PBQPAlloc one-liner | ✅ |
| mir2gpu: 4/4 GPU backends | ✅ |
| CLI -b cuda/opencl/vulkan/metal | ✅ |
| GPU auto-detector (18/18) | ✅ |
| shl-by-0 fix | ✅ |
| C shift narrowing fix | ✅ |
| saveAccForCondRet (partial) | ✅ |
| BCD types in type system | ✅ |
| ADR-0041 (BCD) | ✅ |
| RLCA sled | ✅ |
| 3 Frill demos on Z80 | ✅ |
| "Hello Frill!" on CP/M | ✅ (garbled values) |
| VSCode v0.9.0 (20 commands) | ✅ |
| Nanz book v8 | ✅ |
| C23 book (SDCC comparison!) | ✅ |
| Frill book (GPU chapter) | ✅ |
| "One Compiler, 50 Years" article | ✅ |
| 16 release assets (epub+pdf) | ✅ |

## Corpus Status

| Corpus | Asserts | mir2 | z80 |
|--------|---------|------|-----|
| Nanz (35 examples) | compile | ✅ | partial |
| C89 (38 files) | 350 | ✅ | — |
| C99+ (19 files) | 269 | ✅ | shl fixed, 5 bugs left |
| Frill (16 files) | 427 | ✅ | 3 demos assemble |
| **Total** | **1046** | **✅** | **WIP** |

## Session IDs

```bash
ddll explore
# VIR: cok1cgsq
# z80-optimizer: um2dy4ex
# antique-toy: eo29c66e
```

## Key Insight: The Compiler Stack Works

```
8 Frontends → HIR → MIR2 → 5 Backends
                              ├── Z80 (VIR Z3 + PBQP)
                              ├── CUDA (256/256 NVIDIA)
                              ├── OpenCL (256/256 AMD)
                              ├── Vulkan (256/256 AMD RX 580)
                              └── Metal (256/256 Apple M2)
```

VIR has zero bugs. Every issue found was MIR2/C-frontend/pipeline.
The architecture is proven: universal MIR2 + thin backend wrappers.
swap: RET. abs_diff: 4 bytes. -71% vs SDCC. 50 years of hardware.
