---
name: MIR2 → ABAP backend idea
description: Transpile MIR2 IR to ABAP source code — same program runs on SAP. Vibing-steampunk already has WASM→ABAP (785 LOC), MIR2→ABAP would be simpler (~200-300 LOC) since MIR2 is SSA register-based not stack-based.
type: project
---

MIR2 → ABAP transpiler idea. Since MZV (MIR2 VM) runs all Nanz programs correctly, transpiling MIR2 to ABAP should produce equivalent results on SAP.

**Why:** Run Nanz/ABAP programs natively on SAP without Z80 emulation. Same source, SAP target.

**How:** Like mir2c (MIR2→C) or mir2qbe (MIR2→QBE) but emitting ABAP:
- `%5 = add %3, %4` → `lv_r5 = lv_r3 + lv_r4.`
- `%6 = load %5, u8` → `PERFORM mem_ld_u8 USING lv_r5 CHANGING lv_r6.`
- `call @print, %6` → `PERFORM print USING lv_r6.`
- `brif %7, @then, @else` → `IF lv_r7 <> 0. ... ENDIF.`

**Reference:** vibing-steampunk WASM→ABAP compiler at `~/dev/vibing-steampunk/embedded/abap/wasm_compiler/` (785 LOC ABAP, works on SAP A4H 758).

**How to apply:** When user wants to target SAP, implement `pkg/mir2abap/` backend following mir2c pattern. ABAP has REF TO for pointers, FIELD-SYMBOLS for dereferencing.
