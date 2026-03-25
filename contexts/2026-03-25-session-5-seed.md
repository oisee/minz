# Next Session Seed: ZX Colors + Paper Draft

**Status:** SQL on Z80 CP/M is DONE. Alice | 30, Bob | 25, Charlie | 35.

---

## Completed This Session
- ZSQL SELECT: all rows, both columns, correct order
- MZA pass>=2 fix (strings in binary)
- MZA LD IXH,H fake instruction expansion
- VIR pair aliasing (25/31 Z3, was 18)
- Column index fix (A not C, inline I/O for col 1)
- 3 ZX Spectrum screenshots in README
- GPT-5.4 integration in ddll tested
- Paper A reviewed by GPT-5.4
- Paper B idea: exact inlining via GPU cost oracle

## Next Session Priorities

### 1. ZX Spectrum Colors
Add attribute row colors to ZSQL ZX demo:
- Blue title bar (row 0-1: attr $4F)
- Green SQL commands (rows 3-6: attr $44)
- Cyan SELECT (rows 14-15: attr $45)
- Yellow data rows (rows 19-20: attr $46)
- White headers (row 17: attr $47)

### 2. Real SQL on ZX Spectrum
VIR pair aliasing may fix the ZX spectrum binary (main() was falling to PBQP with BDOS calls). Test: mz mara_alv.abap --vir --target=spectrum

### 3. Paper A Polish
Apply GPT-5.4 review feedback:
- Expand SDCC comparison beyond 5-function micro-benchmark
- Add cost model sensitivity analysis
- Demonstrate on one more architecture (6502?)

### 4. Paper B Prototype
z80-optimizer has the GPU kernel. VIR has call graph. Need:
- DP partitioner (topological sort → bottom-up)
- Merge cost computation from GPU table
- Benchmark: inline decisions on ZSQL corpus

## Session IDs
- VIR: 1oqrq4ku:main
- z80-optimizer: gz3o2bj9:main
- dedelulu: 5d0ib9h6:main
- GPT-5.4: ddll ask gpt54 -s <session>
