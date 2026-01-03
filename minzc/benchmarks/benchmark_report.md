# MinZ Performance Benchmark Report

Generated: Sat Jan  3 12:16:34 PM UTC 2026

## Benchmark Results

### fibonacci

**Description**: Recursive Fibonacci

| Metric | Value |
|--------|-------|
| Source | `fibonacci.minz` |
| Lines | 184 |
| LD | 56 |
| JP/JR | 3 / 6 |
| CALL/RET | 1 / 3 |
| DJNZ | 0 |

### arithmetic_16bit

**Description**: 16-bit arithmetic

| Metric | Value |
|--------|-------|
| Source | `arithmetic_16bit.minz` |
| Lines | 179 |
| LD | 50 |
| JP/JR | 0 / 2 |
| CALL/RET | 1 / 2 |
| DJNZ | 0 |

### nested_loops - FAILED

### memory_operations

**Description**: Memory operations

| Metric | Value |
|--------|-------|
| Source | `memory_operations.minz` |
| Lines | 280 |
| LD | 79 |
| JP/JR | 8 / 4 |
| CALL/RET | 0 / 5 |
| DJNZ | 0 |

### arrays

**Description**: Array access

| Metric | Value |
|--------|-------|
| Source | `arrays.minz` |
| Lines | 103 |
| LD | 27 |
| JP/JR | 0 / 0 |
| CALL/RET | 0 / 1 |
| DJNZ | 0 |

### plasma_simple

**Description**: Plasma effect

| Metric | Value |
|--------|-------|
| Source | `plasma_simple.minz` |
| Lines | 483 |
| LD | 172 |
| JP/JR | 15 / 12 |
| CALL/RET | 2 / 5 |
| DJNZ | 0 |

### plasma_shadow

**Description**: Plasma with shadows

| Metric | Value |
|--------|-------|
| Source | `plasma_shadow.minz` |
| Lines | 810 |
| LD | 317 |
| JP/JR | 27 / 32 |
| CALL/RET | 3 / 8 |
| DJNZ | 2 |


---

## Summary

- **Total**: 7
- **Passed**: 6
- **Failed**: 1

## MinZ vs z88dk/SDCC Advantages

| Feature | MinZ | z88dk/SDCC |
|---------|------|------------|
| Calling convention | Register-based | Stack-based |
| Loop optimization | DJNZ auto | Manual |
| Lambda overhead | Zero | N/A |
| SMC support | Built-in | External |

