# Next Session Seed: SELECT Rows + ZX Spectrum ZSQL

**Goal:** `SELECT * FROM t` displays rows on CP/M. Then ZSQL on ZX Spectrum.

---

## Immediate: SELECT Rows

Two VIR bugs blocking SELECT row display:

1. **INC HL doubled → single**: asm block `INC HL / INC HL` emits only one. getinput() returns _inbuf+1 instead of _inbuf+2. Report to VIR.

2. **_do_select routing**: CP 83 check works in _prompt but _do_select may have wrong parameter setup (tail call without arg forwarding). Check _do_select codegen.

Test:
```bash
printf "CREATE TABLE t(x TEXT)\nINSERT INTO t VALUES('hello')\nSELECT * FROM t\n.quit\n..." | mze ZSQL.COM -t cpm
# Should show: hello | 1 rows
```

## Then: ZX Spectrum ZSQL

ZSQL uses custom puts/putch/readline (BDOS). For ZX Spectrum:
- Replace puts body with _zx_putchar loop (already in ABAP runtime)
- Replace readline with console port $23 input (already done for ABAP)
- Option: import ABAP runtime functions, or add target-conditional asm

## Session IDs
- VIR: cekgp49j:main
- z80-optimizer: x65k8mpc:main
- Us: 25jbcd32:main

## Research
- Phase diagram data ready (Figure 1)
- Corpus transfer 88.2% (Table 1)
- Paper A draft next
