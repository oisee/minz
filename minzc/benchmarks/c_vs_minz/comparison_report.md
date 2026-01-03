# MinZ vs SDCC Z80 Comparison

Generated: Sat Jan  3 12:15:21 PM UTC 2026


## fibonacci

| Metric | SDCC | MinZ | Winner |
|--------|------|------|--------|
| Total Lines | 86 | 137 | SDCC (+59%) |
| Code Lines | 43 | 115 | |
| LD instructions | 8 | 39 | |
| CALL instructions | 2 | 3 | |
| PUSH/POP (stack ops) | 2/2 | 2/2 | |


## arithmetic

| Metric | SDCC | MinZ | Winner |
|--------|------|------|--------|
| Total Lines | 106 | 190 | SDCC (+79%) |
| Code Lines | 45 | 161 | |
| LD instructions | 11 | 52 | |
| CALL instructions | 2 | 0
0 | |
| PUSH/POP (stack ops) | 0
0/0
0 | 2/4 | |


## loop_test

| Metric | SDCC | MinZ | Winner |
|--------|------|------|--------|
| Total Lines | 111 | 194 | SDCC (+74%) |
| Code Lines | 54 | 161 | |
| LD instructions | 14 | 54 | |
| CALL instructions | 1 | 1 | |
| PUSH/POP (stack ops) | 0
0/0
0 | 5/5 | |


---

## Analysis

### SDCC Characteristics
- Stack-based calling convention (push/pop heavy)
- Conservative register allocation
- Portable C semantics

### MinZ Characteristics
- Register-based calling convention
- Z80-optimized register allocation
- Zero-cost abstractions
- SMC (Self-Modifying Code) support

