# Next Session Seed: ZSQL on CP/M + VIR Fixes

**Goal:** `ZSQL.COM` running real SQL queries on CP/M

---

## Blockers (VIR fixes needed)

### 1. ADD DE,HL — 16-bit move pattern
Z3 emits `ADD DE,HL` (invalid Z80). Needs 16-bit move pattern in VIR: `LD D,H / LD E,L`. Reported to VIR session. Affects sap_calculator.abap --vir.

### 2. Blank output on complex programs
sap_hello_zx.abap and zsql.nanz compile clean with VIR (20/25 Z3) but produce no output on CP/M. main() function may have wrong control flow from Z3. Needs VIR investigation.

### 3. GPU regalloc table — 2-arg call patterns
61 entries cover simple leaf functions. Need coverage for `(u16, ^u8)` call patterns (sqlite_exec, sqlite_query). z80-optimizer running GPU kernel.

---

## Ready to Test When VIR Fixes Land

```bash
# ZSQL with VIR
mz examples/nanz/zsql.nanz --vir --target=cpm -o zsql.a80
mza zsql.a80 -o ZSQL.COM
printf "CREATE TABLE t(x TEXT)\nINSERT INTO t VALUES('hello')\nSELECT * FROM t\n.quit\n" | mze ZSQL.COM -t cpm

# Calculator with VIR (tests ADD fix)
mz examples/abap/sap_calculator.abap --vir --target=cpm -o calc.a80
mza calc.a80 -o CALC.COM
echo -e "42\n7\n1" | mze CALC.COM -t cpm
# Should output: ADD => 49 (not 84)

# MARA ALV with VIR (tests full SQLite pipeline)
mz examples/abap/mara_alv.abap --vir --target=cpm -o mara.a80
mza mara.a80 -o MARA.COM
mze MARA.COM -t cpm
```

---

## Also Interesting

- **Frill LSP + VSCode** — user expressed interest
- **Grace MIR2 reroll** — VIR planning store-store-call → loop+table optimization
- **ZX Spectrum colors** — attribute memory works, need to add zx_attr_row calls to ABAP runtime for colored output
