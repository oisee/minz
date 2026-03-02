# Iterator Chain Testing & Hardening Report

**Date:** 2026-02-27
**Commit:** 1db08a7
**Scope:** Iterator chain pipeline — bug fixes, 52 new tests, 14-program regression corpus

---

## Bugs Fixed

### 1. `.iter()` Parser Bug
**File:** `pkg/parser/participle/convert.go`

`tryConvertIteratorChain()` walked backward through method chains but treated `.iter()` as an operation. The `getIteratorOpType("iter")` fell through to the `default` case, returning `IterOpMap` with a nil function argument. This caused every chain starting with `.iter()` to fail with "map requires a function".

**Fix:** Strip `.iter()` as a chain marker — `continue` past it in the walk loop, using the object as the source.

**Impact:** All 6 existing iterator examples now compile (3 were broken).

### 2. DJNZ Counter Persistence Bug
**File:** `pkg/codegen/z80.go` (line ~3274)

The `OpDJNZ` handler emitted a bare Z80 `DJNZ label` instruction. DJNZ atomically decrements B and jumps if non-zero — but the decremented value was never stored back to the virtual register's stack slot. On the next iteration, `loadToB()` re-read the stale original count from the stack, creating an infinite loop.

**Fix:** Replace bare `DJNZ` with manual sequence:
```asm
DEC B           ; decrement counter
LD A, B         ; copy to A
<store to slot> ; persist to stack
JR NZ, label    ; loop if non-zero
```

This costs ~4 extra T-states per iteration but guarantees correctness when the loop body clobbers B (e.g., via CALL instructions in map/filter/forEach).

---

## Test Coverage Added (52 tests)

### Parser Conversion — 13 tests
`pkg/parser/participle/iterator_convert_test.go`

- Chain structure for forEach, map+forEach, filter+forEach, map+filter+reduce
- Take/skip/zip/enumerate operation detection
- `.iter()` stripping (no ops produced)
- Non-iterator methods rejected
- Direct chains without `.iter()` prefix
- All 14 op type mappings verified
- 22 iterator method names + 6 non-iterator rejections

### Semantic Analysis — 13 tests
`pkg/semantic/iterator_test.go`

- DJNZ eligibility for small arrays (5, 10 elements)
- forEach generates OpCall to callback
- map generates call to transform function
- filter generates OpJumpIfNot conditional
- 3-op chain (map+filter+forEach) fused with DJNZ
- Lambda extraction as `iter_lambda_*` functions
- `.iter()` stripping compiles without error
- DJNZ backward branch structure
- Counter initialization to array length
- Element loading via pointer (OpLoad + OpInc)
- Filter continue label placed after body

### Z80 Codegen — 5 tests
`pkg/codegen/iterator_test.go`

- DJNZ emits `DEC B` + `JR NZ` (not bare DJNZ)
- Counter initialization `LD A, N`
- forEach pattern: `CALL` + `DEC B` + `JR NZ`
- Filter conditional jump present
- Store-back persistence (no bare DJNZ in output)

### MIR VM Execution — 7 tests
`pkg/mirvm/iterator_test.go`

- Basic DJNZ loop (5 iterations, accumulator correct)
- Single iteration (count=1)
- Memory store inside loop body
- Nested DJNZ (3×4 = 12 iterations)
- Conditional skip (filter pattern, elements > 3)
- Early exit (take pattern, first 2 elements)
- Counter reaches zero for counts 1, 2, 5, 10, 255

### Regression Corpus — 14 programs
`tests/iterator_corpus/*.minz` + `iterator_corpus_test.go`

Each program compiled through full pipeline (parse → semantic → Z80 codegen):

| Program | Pattern |
|---------|---------|
| `iter_basic_foreach` | `arr.forEach(fn)` |
| `iter_map_double` | `arr.map(double).forEach(fn)` |
| `iter_filter_gt5` | `arr.filter(is_big).forEach(fn)` |
| `iter_map_filter` | `arr.map(f).filter(g).forEach(h)` |
| `iter_lambda_map` | `.iter().map(\|x\| x+1).forEach(fn)` |
| `iter_lambda_filter` | `.iter().filter(\|x\| x>3).forEach(fn)` |
| `iter_iter_forEach` | `.iter().forEach(fn)` |
| `iter_map_filter_forEach` | `.iter().map().filter().forEach()` full chain |
| `iter_direct_methods` | `arr.map(f).forEach(g)` without `.iter()` |
| `iter_multi_function` | Chains in multiple functions |
| `iter_single_element` | Edge case: array[1] |
| `iter_lambda_chain` | Mixed lambda + function ref |
| `iter_range_loop` | `for i in 0..5` |
| `iter_peek_inspect` | `.peek(log).forEach(print)` |

---

## Pipeline Status After This Work

| Stage | Status | Coverage |
|-------|--------|----------|
| Parser → IteratorChainExpr | ✅ Working | 13 tests |
| `.iter()` stripping | ✅ Fixed | 3 tests |
| Semantic → DJNZ generation | ✅ Working | 6 tests |
| Semantic → lambda extraction | ✅ Working | 2 tests |
| Semantic → filter labels | ✅ Working | 2 tests |
| Codegen → Z80 DEC B/JR NZ | ✅ Fixed | 5 tests |
| MIR VM → OpDJNZ execution | ✅ Working | 8 tests (7 new + 1 existing) |
| Full pipeline regression | ✅ 14/14 pass | 14 programs |

### Remaining Gaps (not addressed)
- **Fusion optimizer** (`pkg/optimizer/fusion.go`): Still skeleton-only. All passes return nil.
- **Enhanced ops** (skip/take/enumerate/reduce): Semantic analysis exists but `test_enhanced_skip_take.minz` had separate failures (nil function in enhanced path). Not blocking basic chains.
- **String iteration**: Returns "not yet implemented" error.
- **LDIR/LDDR inside DJNZ loops**: Not yet protected (BC clobbering). Mitigated by the store-back fix but not optimal.

---

## How to Run

```bash
cd minzc

# All iterator tests
go test ./pkg/parser/participle/ -run "TestIterator|TestNon|TestIterAlone|TestIsIterator" -v
go test ./pkg/semantic/ -run "TestDJNZ|TestForEach|TestMap|TestFilter|TestIter|TestElement" -v
go test -vet=off ./pkg/codegen/ -run "TestDJNZ|TestIterator|TestFilter" -v
go test ./pkg/mirvm/ -run "TestDJNZ" -v
go test -vet=off ./tests/iterator_corpus/ -v
```
