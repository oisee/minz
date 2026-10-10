# Grace: pointer conformance and corpus gap closure

**Update:** [Scalar pointer types from source through VM/Z80](grace-typed-pointers.md) are now implemented and checked. The earlier status below is retained as a snapshot.

2026-10-10. Branch `feat/grace-hir-vm`, follow-up to the HIR/MIR audit.

## Result

The same `TestGraceVerify` corpus now passes **52/52**, versus **48/52** in
commit `0db3b115`. This is the existing corpus with its existing assertions;
no failing examples or assertions were removed. Passing this corpus is not a
proof of compiler correctness or of equivalence on all inputs.

The complete `pkg/mir2`, `pkg/hir`, and `pkg/pipeline` package tests pass.
Captured logs are in [gap-closure-artifacts](grace-gap-closure-artifacts/tests.txt)
and [corpus](grace-gap-closure-artifacts/corpus.txt).

| Gap | Cause | Implemented fix |
|---|---|---|
| VM pointer arithmetic depends on copies/CFG | Pointer producers kept unbounded int64 while typed arguments/moves truncated | Truncate pointer results at production, including PtrAdd/Field/PtrBump and pointer-returning calls |
| VM vs Z80 at the address boundary | Multi-byte VM access used contiguous heap addresses | Wrap each byte address at the source pointer's width; preflight all store bytes before writing |
| SoA256 advance differs between VM and Z80 | Legacy immediate −1 means increment low byte in Z80 | VM now uses the same legacy layout operation; it is not ordinary pointer subtraction |
| Verifier accepts sibling-block values or duplicate SSA definitions | Global definition set followed block-list order | Collect all definitions, reject duplicates/local forward references, check dominance in reachable CFG |
| `18_tail_recursion.nanz` returns fib(1)=0 | Grace replaced function parameters only in the former entry block | Remap uses in every body/successor block; retain recursive attribute when a self call remains |
| Two screen examples fail edge arity | Redundant-load forwarding appended a loop-header parameter but supplied it only on one incoming edge | Require a unique supplying predecessor; reject self/entry forwarding, opaque/memory-changing blocks, and differing load types |
| `hello_cpm_fib.nanz` returns gcd(12,8)=8 | Unsigned division assigned B=dividend before copying a divisor already in B | Set up B/C using parallel copies |

## Independent checks

- VM pointer arithmetic: 16/24-bit wrap, observed directly, through a move and
  a block edge. A 24-bit pointer in a direct call's extra return retains its width.
- VM memory: ordinary/patched loads and stores across the address-space end,
  distinct store sentinels, and atomic failure on a partial heap. Huge 64-bit
  addresses still produce diagnostics rather than panicking.
- Native Z80 vs VM: +1 at 65535, indexed byte load, and explicitly typed HIR
  u16 load/store crossing 65535→0. Both ordinary and Grace pipelines pass.
  The fixture uses all-RAM memory; ZX Spectrum ROM permissions are a separate
  platform policy and must not be confused with address wrap.
- Existing 15,360-case HIR pointer-walk differential matrix now uses an
  independent modulo-65536 address oracle. Invariant-base differential tests pass.
- Decimal parser: empty input, digits, nondigit suffix and u16 overflow,
  ordinary and Grace MIR routes. Fibonacci: n=0..30 modulo 256 on both routes.
- Native unsigned quotient/remainder/GCD: eight input pairs on both routes,
  including reversed register dependencies and large byte operands.
- Redundant-load safety: independent expected values across an intervening
  store and differing load widths; safe forwarding still fires.

## Scope and next gates

The guarded HIR pointer-walk remains opt-in; its guard has not been removed.
Existing byte/T-state measurements describe the guarded implementation and are
not reinterpreted as measurements of a future unconditional rewrite.
`--Osize` already disables that expanding rewrite while preserving invariant-base.

| Remaining item | Status / next action |
|---|---|
| Scalar pointee types in Nanz | `parseType`/`parseParamType` discard ^u16's element type; postfix dereference defaults to u8. Native boundary tests deliberately construct typed HIR for u16 instead of claiming source-level coverage. Preserve pointee information through parameters, locals, expressions and calls before extending these claims. |
| Target selection, including eZ80 | Typed MIR/VM 24-bit cases are tested. This does not implement an eZ80 code generator, ADL mode or frontend-wide pointer-width selection. Wire target data layout end to end. |
| Host C/QBE and SoA256 | Host addresses are not automatically 16-bit emulated addresses. Resolve target-memory vs host-pointer models and legacy SoA encoding before claiming cross-backend conformance. |
| Strict MIR type checking | Dominance and unique definitions are implemented; width-changing block edges still use existing VM numeric conversion semantics. Define and migrate a typed edge contract before enforcing strict equality. |
| HIR verification | Structural rewrites still rely on MIR verification after lowering and differential tests; a dedicated typed HIR verifier remains to implement. |
| Pointer-walk profitability | Remove the guard only after extending conformance coverage; then remeasure short/long inputs, bytes, T-states and compile cost. u8 index overflow still separately blocks generic MIR threading. |
| VM allocation attribution | The earlier allocation delta has not been attributed with an allocation profile. New frame pointer metadata is also included in future performance measurements. |
| LLM-less PGO + Grace | ADR-0046 records the architecture; collection, stable site IDs, profile validation and guarded specialization are not implemented. Observations select profitable candidates; static proof or runtime guards supply correctness. |
| LIR migration | Selected migrations still require a real production gap and target byte/T-state measurements. No wholesale migration is claimed. |

These are explicit remaining tasks, not hidden exceptions to the 52/52 result.
