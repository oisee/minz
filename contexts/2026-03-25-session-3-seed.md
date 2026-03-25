# Next Session Seed: Direct PIR Emit + ZSQL on CP/M

**Goal:** ZSQL.COM executing real SQL queries on CP/M

---

## Immediate: VIR Direct PIR Emit (cekgp49j)

The last piece. GPU table has 25+ real corpus entries with width-aware assignments. VIR needs:

1. On table hit: skip Z3, iterate ~40 patterns per op, find pattern where dst/src locs match GPU assignment
2. Emit PIR directly from matched patterns + GPU locs
3. No solver, no satisfiability check — GPU already proved it's optimal

Test targets:
```bash
# Calculator — tests ADD u16 (was doubled on production, ADD DE,HL on old VIR)
mz examples/abap/sap_calculator.abap --vir --target=cpm -o calc.a80
echo "42\n7\n1" | mze CALC.COM -t cpm
# Expected: ADD => 49

# ZSQL — tests sqlite_exec/query string pointer preservation
mz examples/nanz/zsql.nanz --vir --target=cpm -o zsql.a80
printf "CREATE TABLE t(x TEXT)\nINSERT INTO t VALUES('hello')\nSELECT * FROM t\n.quit\n" | mze ZSQL.COM -t cpm
# Expected: OK / OK / hello / 1 rows

# MARA ALV — tests full Open SQL pipeline
echo "" | mzv -H examples/abap/mara_alv.abap
# Already works on mzv — test on CP/M with VIR
```

---

## GPU Regalloc Table Expansion

z80-optimizer kernel ready with width support (commit 2089c42). To expand table:
```bash
# Extract real signatures from corpus
VIR_DUMP_GPU_BATCH=1 mz --vir corpus/*.nanz > sigs.jsonl
# Solve on GPU
gpu-server < sigs.jsonl > results.jsonl
# Merge into table
# (VIR has merge tool)
```

---

## Also Ready

- **ZX Spectrum ZSQL** — needs ZX puts/putch variants (swap BDOS for _zx_putchar)
- **ABAP ZX colors** — attribute rows work, add to ABAP runtime for colored output
- **Frill LSP + VSCode** — user interested, deferred
- **Grace MIR2 reroll** — VIR planning store-store-call → loop+table at MIR level

---

## Session IDs (may change)
- VIR: cekgp49j:main (minz-vir)
- z80-optimizer: x65k8mpc:main
- Us: 25jbcd32:main (minz)
