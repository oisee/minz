# Next Session Seed — 2026-03-30

**Previous:** Session 16 — GPU arithmetic library, scalar op overload, carry_compare, SHA-256
**State:** 8 commits, ~800 GPU entries, intrinsics.go live, fun/ playground

---

## Priority 0: Move fun/ Demos → stdlib/ Importable Modules

Currently fun/vectors.nanz, fun/raymarcher.nanz are self-contained.
Need:
- `stdlib/math/vec.nanz` — Vec2, Vec3 with operator overloading + impl
- `stdlib/math/wide.nanz` — widening multiply, sat_add, sat_sub, clamp
- `stdlib/graphics/sdf.nanz` — SDF primitives, raymarcher
- Register in import resolver so `import math.vec` works

## Priority 1: Wire Remaining GPU Tables into intrinsics.go

4tw49890's intrinsics.go has div8 + mod8. Still need:
- Widening mul (OpMulWiden): `LD L,A; LD H,0;` + mul16[K]
- sat_add8 (16T!): `ADD A,B; LD C,A; SBC A,A; OR C`
- sat_sub8 (20T): `SUB B; LD C,A; SBC A,A; CPL; AND C`
- abs8 (24T), sign8 (43T), min8/max8 (32T)
- abs16 (44T), neg16 (27T), min16/max16 (41T)
- u32 ops: SHL32, SHR32, ADD32, NEG32

Pattern is established: tryConst* → OpAsmBlock → Z3 allocates around.

## Priority 2: Fix Self-Hosting Bugs (from session 15)

1. **Multi-function emit_lanz**: first function outputs nulls to buffer
2. **print_ast infinite recursion**: AST traversal cycles

## Priority 3: FatFS VIR_DUMP_GPU_BATCH

Corpus bias: our functions max 14v, FatFS 35v. Need dump for GPU regalloc calibration.

## Priority 4: MIR2 VM Struct Fixes

- Struct return field access causes OOB (heap too small)
- This blocks fun/vectors.nanz from testing struct operators via MIR2 asserts

---

## What Was Done in Session 16

| Feature | Status |
|---------|--------|
| Scalar operator overloading (parse.go) | ✅ |
| GPU-optimal mul16 in codegen (254 entries, 7.7×) | ✅ |
| Mul16OptTable loader | ✅ |
| U32OpsTable loader (13 ops) | ✅ |
| DivOptTable loader (254 entries, v3) | ✅ |
| div8 carry_compare (GPU-discovered, 26T) | ✅ verified 32768/32768 |
| intrinsics.go — tryConstDiv/tryConstMod | ✅ (4tw49890) |
| widemath.nanz (31 asserts) | ✅ |
| SHA-256 primitives (808 bytes, 15 funcs) | ✅ |
| MZA accepts .asm/.z80 extensions | ✅ |
| ^ = pointer deref bug found + fixed | ✅ |
| fun/ playground (raymarcher, vectors, README) | ✅ |
| Report + README featured section | ✅ |
| CLAUDE.md GPU arithmetic library section | ✅ |

## GPU Tables Available (from z80-optimizer)

| Table | Entries | File | Codegen |
|-------|---------|------|---------|
| mul8 A×K→A | 254 | mulopt8_clobber.json | ✅ VIR pipeline |
| mul16 HL×K→HL | 254 | mulopt16_complete.json | ✅ VIR pipeline |
| div8 A÷K→A (v3) | 254 | div8_optimal.json | ✅ intrinsics.go |
| mod8 A%K→A | 254 | mod8_optimal.json | ✅ intrinsics.go |
| divmod8 | 254 | divmod8_optimal.json | loaded |
| u32 ops | 13 | u32_ops.json | loaded |
| sign8/sat8 | 3+1 | sign_sat_ops.json | loaded |
| arith16 | 6 | arith16_new.json | loaded |

## Team Status

| Session | Role | Status |
|---------|------|--------|
| z80-optimizer (um2dy4ex) | GPU bruteforce | sleeping, all tables delivered |
| minz-vir (4tw49890) | VIR backend | on-call, intrinsics.go ready for more |
| minz-abap (gyfiwji1) | LLVM backend | 42→77 projected, runtime stubs next |
| antique-toy (oy1tl7nn) | steampunk game | FatFS→ABAP done |

## Key Discoveries to Remember

- `^` in Nanz = pointer deref (postfix), `xor` keyword = bitwise XOR
- ADC HL,rr exists (ED prefix, 15T) — enables efficient u32
- sat_add8 = ADD A,B; LD C,A; SBC A,A; OR C (4 insts, 16T, branchless masterpiece)
- carry_compare: OR A; LD B,(256-K); ADC A,B; SBC A,A; AND 1 (div K≥128, 26T)
- GPU brute-force finds tricks humans and textbooks miss
- Three-level validation: analytical → compositional → GPU exhaustive
- fun/ demos are NOT importable — need stdlib/ migration
- Nanz assert syntax: wrap struct expressions in functions, can't use inline struct literals
- MIR2 VM: struct return + field access = OOB (heap sizing)
