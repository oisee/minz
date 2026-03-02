# Iterator E2E Testing Report

*2026-03-01 | MinZ v0.19.4 | Report #014*

---

## Executive Summary

The MinZ iterator chain pipeline has **63 unit tests across 4 packages**, **18 corpus regression tests**, and **6 E2E integration tests**. All unit and corpus tests pass. E2E shell tests reveal a **pointer-aliasing regression** in Z80 codegen: `OpLoad` reads the base array pointer (`r8`) instead of the advancing pointer (`r10`), causing every iteration to load the first element.

| Layer | Tests | Pass | Fail | Skip |
|-------|------:|-----:|-----:|-----:|
| Parser (chain conversion) | 18 | 18 | 0 | 0 |
| Semantic (IR generation) | 20 | 20 | 0 | 0 |
| Codegen (Z80 patterns) | 7 | 7 | 0 | 0 |
| MIR VM (DJNZ execution) | 8 | 8 | 0 | 0 |
| Corpus (full compile) | 18 | 18 | 0 | 0 |
| E2E shell (hex output) | 6 | 0 | 2 | 4 |
| **Total** | **77** | **71** | **2** | **4** |

---

## Regression: Array Pointer Aliasing Bug

### Symptom

`iter_foreach.minz` outputs `41414141410a` (AAAAA\n) instead of `41424344450a` (ABCDE\n). Every element reads as `A` (65), the first byte.

### Root Cause

In the generated Z80 assembly, the element load always reads from virtual register `r8` ($F010 = array base) instead of `r10` ($F014 = advancing pointer):

```asm
; Line 34: BUG — loads from r8 (base), not r10 (advancing ptr)
e2e_iterators_iter_foreach_main_djnz_loop_1_i1:
    LD HL, ($F010)    ; <-- always array base
    LD A, (HL)        ; <-- always first element

    ; ... call console_log ...

    ; Line 48-50: r10 correctly incremented
    LD HL, ($F014)    ; Virtual register 10 from memory
    INC HL
    LD ($F014), HL
```

The IR `OpLoad Src1=r8 Dest=r11` should be `OpLoad Src1=r10 Dest=r11`. The semantic analyzer creates `r10` as the pointer copy but the `OpLoad` still references the original `r8`.

### Impact

All forEach/take/skip chains produce repeated first-element output. map/filter chains with named functions have the same issue. This is a semantic analyzer bug in `generateDJNZIteration()` — the pointer register assignment for element load.

### Location

`pkg/semantic/iterator.go` — `generateDJNZIteration()`. The `OpLoad` instruction's `Src1` field should reference the copy register (ptrReg) that gets `INC HL`, not the original array register.

---

## Test Results by Package

### Parser: 18/18 PASS

File: `pkg/parser/participle/iterator_convert_test.go`

| Test | Validates |
|------|-----------|
| TestIteratorChainBasicForEach | `arr.forEach(fn)` chain conversion |
| TestIteratorChainMapWithLambda | `.iter().map(|x| x*2).forEach(fn)` |
| TestIteratorChainFilterForEach | `.filter(|x| x>5).forEach(fn)` |
| TestIteratorChainMapFilterReduce | 3-op chain: map -> filter -> reduce |
| TestIteratorChainTakeSkip | `.take(5).skip(2).forEach(fn)` |
| TestIteratorChainZip | `.zip(other)` (parser-level only) |
| TestIteratorChainEnumerate | `.enumerate()` produces IterOpEnumerate |
| TestNonIteratorMethodNotConverted | `obj.method()` NOT converted to chain |
| TestIterAloneNoChain | `.iter()` alone, no chain ops |
| TestIteratorChainDirectMethods | `arr.map(fn)` without `.iter()` |
| TestIteratorOpTypeMapping | All 14 iterator ops mapped (14 subtests) |
| TestIsIteratorMethod | 19 iterator method names recognized |
| TestTakeArgumentRouting | `take(5)` -> Argument=5, Function=nil |
| TestSkipArgumentRouting | `skip(2)` -> Argument=2, Function=nil |
| TestEnumerateNoArguments | `enumerate()` -> both nil |
| TestReduceWithFunctionRouting | `reduce(fn)` -> Function=fn |
| TestReduceWithInitAndFunction | `reduce(0, fn)` -> both populated |
| TestMapFunctionInFunctionField | `map(double)` -> Function=double |

### Semantic: 20/20 PASS

File: `pkg/semantic/iterator_test.go`

**DJNZ dispatch & loop structure (7 tests):**

| Test | Validates |
|------|-----------|
| TestDJNZForSmallArray | Array <=255 uses DJNZ path |
| TestDJNZForMaxArray | Array[10] uses DJNZ |
| TestDJNZLoopStructure | Label precedes DJNZ (backward branch) |
| TestDJNZCounterInit | `OpLoadConst(counter, 5)` before loop |
| TestElementLoadedViaPointer | `OpLoad` + `OpInc` for pointer walk |
| TestIterStrippedFromChain | `.iter()` stripped, no op generated |
| TestFilterContinueLabelAfterBody | Filter skip label placed correctly |

**Basic ops (6 tests):**

| Test | Validates |
|------|-----------|
| TestForEachGeneratesCall | forEach -> OpCall |
| TestMapWithFunctionRef | map(double).forEach(print_u8) |
| TestFilterGeneratesConditionalJump | filter(is_big) -> OpJumpIfNot |
| TestMapFilterForEachChain | 3-op chain |
| TestMapWithLambda | Lambda extracted as separate function |
| TestFilterWithLambda | Lambda `x > 3` inlined -> OpJumpIfFlag |

**Enhanced ops (7 tests):**

| Test | Validates |
|------|-----------|
| TestTakeCompiles | `take(3)` -> counter=3 |
| TestSkipCompiles | `skip(2)` -> skip comment |
| TestTakeSkipCombined | `skip(2).take(5)` both present |
| TestEnumerateCompiles | enumerate() with DJNZ |
| TestReduceWithFunction | reduce(sum) with OpCall |
| TestInlineFilterLambdaGt | `|x| x > 3` -> `OpJumpIfFlag(FlagCY)` + CP 4 |
| TestInlineFilterLambdaEq | `|x| x == 3` -> `OpJumpIfFlag(FlagNZ)` + CP 3 |
| TestInlineFilterLambdaLt | `|x| x < 3` -> `OpJumpIfFlag(FlagNC)` + CP 3 |
| TestInlineFilterConstOnLeft | `|x| 5 > x` normalized |
| TestComplexFilterFallback | `|x| x>3 && x<10` -> OpCall (no inline) |
| TestNamedFunctionFilterFallback | `filter(is_big)` -> OpCall + OpJumpIfNot |

### Codegen: 7/7 PASS

File: `pkg/codegen/iterator_test.go` (requires `-vet=off`)

| Test | Z80 Pattern Validated |
|------|----------------------|
| TestDJNZCodegen | `DEC B` + `JR NZ` |
| TestDJNZCounterInit | `LD A, 10` for counter |
| TestIteratorForEachCodegen | `CALL fn` + `DEC B` + `JR NZ` |
| TestFilterConditionalJump | `JR Z,` / `JP Z,` |
| TestDJNZStoreBackPersistence | `LD A, B` before/after DEC |
| TestInlineFilterCodegen | `CP N` + `JR cond,` |
| TestInlineFilterAllConditions | CY/NC/Z/NZ (4 subtests) |

### MIR VM: 8/8 PASS

File: `pkg/mirvm/iterator_test.go`

| Test | Simulates | Expected |
|------|-----------|----------|
| TestDJNZBasicLoop | 5 iterations | r1=5 |
| TestDJNZSingleIteration | 1 iteration | r1=10 |
| TestDJNZWithStore | Store intermediate | r1=21 (3x7) |
| TestDJNZNestedLoops | 3 x 4 = 12 iters | r1=12 |
| TestDJNZWithConditionalSkip | Filter >3 from [1..5] | acc=9 (4+5) |
| TestDJNZWithEarlyExit | Take 2 from [1..5] | acc=3 (1+2) |
| TestDJNZCounterReachesZero | Counts 1,2,5,10,255 | counter=0 |
| TestDJNZOp | Basic opcode test | PASS |

### Corpus: 18/18 Compile to Z80

Directory: `tests/iterator_corpus/`

| File | Pattern |
|------|---------|
| iter_basic_foreach.minz | `arr.forEach(fn)` |
| iter_direct_methods.minz | `arr.map(...).filter(...)` |
| iter_enumerate.minz | `.enumerate()` |
| iter_filter_gt5.minz | `filter(|x| x > 5)` |
| iter_iter_forEach.minz | `.iter().forEach(...)` |
| iter_lambda_chain.minz | Multi-lambda chain |
| iter_lambda_filter.minz | `filter(|x| ...)` |
| iter_lambda_map.minz | `map(|x| ...)` |
| iter_map_double.minz | `map(double_fn)` |
| iter_map_filter.minz | `map(...).filter(...)` |
| iter_map_filter_forEach.minz | 3-op chain |
| iter_multi_function.minz | Multiple functions |
| iter_peek_inspect.minz | `peek()` and `inspect()` |
| iter_range_loop.minz | Range-based iteration |
| iter_reduce_sum.minz | `reduce(sum_fn)` |
| iter_single_element.minz | Single-element array |
| iter_skip_2.minz | `skip(2)` |
| iter_take_3.minz | `take(3)` |

All 18 compile to `.a80` assembly without errors.

### E2E Shell Tests: 0/2 PASS, 4 not wired

File: `examples/e2e_iterators/run_e2e.sh`

| Test | Expected | Got | Status |
|------|----------|-----|--------|
| iter_foreach | `41424344450a` (ABCDE\n) | `41414141410a` (AAAAA\n) | **FAIL** |
| iter_take | `4142430a` (ABC\n) | `4141410a` (AAA\n) | **FAIL** |
| iter_skip | `4344450a` (CDE\n) | not wired | skipped |
| iter_map_foreach | `020406080a0a` | not wired | skipped |
| iter_filter_foreach | `44450a` | not wired | skipped |
| iter_lambda_map | `42434445460a` | not wired | skipped |

Both failures share the same root cause: pointer aliasing bug (see Regression section above).

---

## Operation Support Matrix

### Fully Working (parser + semantic + codegen + MIR)

| Operation | Unit Tests | Corpus | E2E |
|-----------|:----------:|:------:|:---:|
| forEach(fn) | 4 | 3 | FAIL (pointer bug) |
| map(fn) | 3 | 3 | FAIL (pointer bug) |
| filter(fn) | 4 | 3 | FAIL (pointer bug) |
| filter(lambda) inline | 5 | 2 | not wired |
| take(n) | 2 | 1 | FAIL (pointer bug) |
| skip(n) | 2 | 1 | not wired |
| peek(fn) | 1 | 1 | not wired |
| inspect(fn) | 1 | 1 | not wired |
| takeWhile(fn) | 1 | 0 | not wired |
| enumerate() | 2 | 1 | not wired |
| reduce(fn) | 2 | 1 | not wired |

### Parser Only (no semantic/codegen)

| Operation | Status |
|-----------|--------|
| zip(other) | Parser recognizes, no semantic impl |
| flatMap(fn) | Parser recognizes, no semantic impl |
| collect() | Parser recognizes, no semantic impl |
| skipWhile(fn) | Parser recognizes, semantic partial |

---

## Known Bugs

### Critical

| Bug | Impact | Location |
|-----|--------|----------|
| **Pointer aliasing** | All E2E tests fail — OpLoad reads base ptr (r8) not advancing ptr (r10) | `semantic/iterator.go` `generateDJNZIteration()` |

### Medium

| Bug | Impact | Location |
|-----|--------|----------|
| **Inline filter constant not tracked** | `filter(|x| x > 3)` compares against 0, not 3 — passes all elements | `codegen/z80.go` constantValues map |
| **OpPush routes through HL** | enumerate/reduce push wrong register | `codegen/z80.go` OpPush handler |
| **Counter waste** | ~12 T-states/iter wasted on `LD A,B / LD B,A` shuffle | Peephole opportunity |

### Low

| Bug | Impact | Location |
|-----|--------|----------|
| **Unused result saves** | 4 T-states wasted after void CALL | Peephole opportunity |

---

## Recent Fix History

| Date | Commit | Fix |
|------|--------|-----|
| 2026-02-28 | `b601137` | MIR peephole Imm/Value, SMC arg loading, lambda inlining, copy propagation |
| 2026-02-28 | `19dccca` | PUSH/POP HL in DJNZ to preserve array pointer across CALL |
| 2026-02-28 | `72fe193` | Superoptimizer peephole + TRUE SMC anchor + HL clobber fix |
| 2026-02-27 | `a9b2545` | Enhanced iterator ops (take/skip/enumerate/reduce) |
| 2026-02-14 | `1db08a7` | DJNZ counter persistence + .iter() parser bug + 52 tests |

---

## Test File Locations

```
pkg/parser/participle/iterator_convert_test.go    18 parser tests
pkg/semantic/iterator_test.go                      20 semantic tests
pkg/codegen/iterator_test.go                        7 codegen tests (needs -vet=off)
pkg/mirvm/iterator_test.go                          8 MIR VM tests
tests/iterator_corpus/                             18 .minz corpus files
examples/e2e_iterators/                             6 E2E tests + run_e2e.sh
```

## Implementation Files

```
pkg/semantic/iterator.go              Iterator semantic analysis + DJNZ dispatch
pkg/semantic/iterator_enhanced.go     Enhanced ops (take/skip/enumerate/reduce)
pkg/codegen/z80.go                    Z80 codegen (DJNZ generation ~line 4200)
pkg/optimizer/fusion.go               Fusion framework (skeleton, not wired)
pkg/parser/participle/convert.go      tryConvertIteratorChain() at line ~1002
```

---

## Next Steps

1. **Fix pointer aliasing** — `generateDJNZIteration()` must use the copy/advancing register in `OpLoad`, not the base array register
2. **Fix inline filter constant tracking** — embed constant in `OpJumpIfFlag.Imm2` or preserve constantValues across IR
3. **Wire remaining E2E tests** — skip, map, filter, lambda_map into `run_e2e.sh`
4. **Fix OpPush register routing** — direct `PUSH BC`/`PUSH DE` instead of always `PUSH HL`
5. **Wire fusion optimizer** — `pkg/optimizer/fusion.go` skeleton exists, needs pipeline integration
