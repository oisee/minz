# Nanz integer literal typing (P28)

Integer tokens, including a leading unary minus, have a mathematical value and
an untyped-literal marker in HIR. Their default u8/u16/u24/u32 (or signed
counterpart for negative values) is a storage choice, not a vote on the
signedness of an expression. Enum members, explicitly cast expressions, and
literals supplied by other frontends are typed values.

For binary integer arithmetic, bitwise operations, shifts, and comparisons,
select the operation domain before lowering either operand:

| Operands | Domain |
|---|---|
| One untyped literal that fits the typed peer | The peer's type, including signedness |
| One untyped literal that does not fit | Smallest integer type containing the literal value and the peer's entire domain |
| A byte/word peer would need a common type wider than 16 bits | Compile error requiring an explicit wide cast |
| Two untyped literals | Smallest unsigned type containing both values; signed if either value is negative |
| Two typed expressions | Existing typed-expression rules; explicit casts remain authoritative |
| Comparison | bool result; operands use the domain above |

For byte/word peers, the concrete decision table is:

| Typed peer | Literal values | Operation domain |
|---|---|---|
| i8 | -128..127 | i8 |
| i8 | -32768..-129 or 128..32767 | i16 |
| u8 | 0..255 | u8 |
| u8 | -32768..-1 | i16 |
| u8 | 256..65535 | u16 |
| i16 | -32768..32767 | i16 |
| u16 | 0..65535 | u16 |
| Any byte/word peer | Other values | Error: explicit wide cast required |

Candidate widths are 8, 16, 24, 32. Widening preserves signedness for a signed
peer. A negative literal with an unsigned peer requires a signed type with an
extra bit for the unsigned peer's range. Operand order does not affect type
selection. This is range-preserving conversion rather than C's blanket `int`
promotion: byte operations retain byte wraparound.

Examples: `1 + x:i8` computes in i8; `256 + x:i8` in i16;
`x:i16 + 256` in i16; `-1 + x:u8` in i16. Thus `256 >= x:i8` and
`x:i8 <= 256` agree, including at -128. Commutative operations agree when
swapped; subtraction, shifts and comparisons retain their ordinary ordered
meaning (reverse a comparison predicate when swapping its operands).

Arithmetic results wrap at the selected width. Right shift is arithmetic for
a signed operation domain and logical otherwise. Shift counts must be
nonnegative and smaller than the operation width, as for defined C shifts.

An initializer, assignment, call parameter, or return provides a destination
context. A fitting literal is interpreted as that type. A non-fitting value is
first represented without loss and then undergoes the existing destination
integer conversion (low bits on narrowing, signed extension on signed
widening). For example, assigning -1 to u16 yields 65535. A destination does not
retroactively widen a typed binary subexpression: `let y:u16 = x:u8 + 1`
retains byte wraparound before conversion. Explicit `as` casts request a
conversion and end literal inference.

`hir.IntLiteralFits` and the shared integer-domain selector implement these
rules; `hir.RangeType` uses the same selector for range bounds. Lowering may
materialize a nonnegative constant in a narrower unsigned register when its
value is unchanged, preserving existing efficient assembly. C HIR supplies its
own conversions and has no Nanz untyped-literal marker.

The generated pipeline property test checks mathematical-domain selection and
executes boundary inputs through raw MIR2 and optimized MIR2/Z80. Shift tests
exclude counts outside the defined domain. Division/remainder tests exclude zero
divisors and signed MIN/-1, where C division is undefined.

If no supported type can contain the domain (for example, u32 with -1, or i32
with 4294967295), parsing reports an error and asks for an explicit cast.
The same check runs after forward function signatures have been resolved.
Operator overload lookup uses the contextual operand types as well.
Integer arithmetic containing an unresolved forward call stays unresolved
until the signature repair pass. Inferred locals and nested expressions then
receive the resolved peer type; an explicit local annotation remains binding.
Known pointer arithmetic keeps its established pointer result type.

Implicit widening of a byte/word peer stops at 16 bits. For example, i16 with
32768 or -32769, and u16 with -1 or 65536, require an explicit wide cast. The
property test checks rejection for both operand orders. This keeps implicit
literal inference within the byte/word backend paths: a generated probe exposed
an existing Z80 wide-shift failure for `-32769 >> x:i16` at x=1 (MIR2 49151 after
conversion to u16, Z80 16383). Explicit wide operations retain their existing
backend behavior and are outside this change's byte/word inference guarantee.

Exact source templates in the rewrite proof checkers carry the source literal
marker too. Their generated rewrite nodes continue to use explicitly typed
constants; literal values, widths, statement order, origins, and other proof
metadata remain part of the existing strict comparisons.

Ranges retain P26's additional admission rules: wider signed bounds may select
i24/i32, and a negative literal paired with an unsigned typed bound requires an
explicit signed induction type. Their literal fit checks and mathematical
selection still use the shared helpers. Range lowering does not introduce the
general wide arithmetic/shift combinations rejected for binary inference.

## Validation (2026-10-22)

The before worktree is detached `origin/main` at `c5b0065eb`. The final test
source was copied there unchanged. `TestNanzLiteralTypingPropertyVMZ80/i8/+`
fails with `p6: type u8 want 8 signed=true`; the changed compiler passes.
`TestNanzLiteralTypingContextsVMZ80/signed_division` also fails on that baseline
at -32767 (`got 0, want -15`) and passes on the changed compiler.
The `forward_initializer` subtest fails on that baseline at x=-128
(`got 0, want 1`) and passes after deferring unresolved arithmetic inference.

The generated property test covers i8/u8/i16/u16, both operand orders, boundary
literals and inputs, arithmetic including division/remainder, bitwise operators,
shifts, and all six comparisons. It checks HIR/MIR types and predicates before
running raw MIR2, optimized MIR2, and Z80. All 64 type/operator groups and the
context/diagnostic tests pass; the two new tests together take 3.388 seconds.
Each native module and its input trampolines are assembled once.

The required package command passed:

```sh
cd minzc
nice -n 19 go test ./pkg/mir2/... ./pkg/hir/... ./pkg/c89/... ./pkg/nanz/... ./pkg/z80asm/... -count=1
```

Both new tests and the eight selected existing pipeline regressions passed.
The complete pipeline package was not run. Both new top-level names are in the
sorted `fast` inventory. Direct-call judge seeds 0–499 passed 500/500; folded
seeds 0–99 passed 100/100, each with four workers and no mismatches, assembly
failures, compiler errors, or oracle errors. Heavy commands used `nice -n 19`,
the requested SSD TMPDIR/GOCACHE, at most four jobs, and the load <=12 gate with
120-second retries. No heldout suite or assert_matrix was run.

The 154-file examples/stdlib Nanz corpus has the same 144 successful builds and
10 pre-existing failures before and after. Of the successful builds, 143 emit
byte-for-byte identical assembly. The sole changed file is
`examples/nanz/rotozoomer.nanz`, which contains signed `(u / 256) % 16` and the
same expression for v. Its old unsigned division returned 14 at u=-257; signed
truncation and remainder require -1. The new signed sequence is therefore a
correctness change for the declared i16 input domain, not an inferred unsigned
range. The image changes from 2622 to 2772 bytes; `draw_frame` changes from 679
to 829 bytes (+150). The generic signed-chain regression above checks both
engines. No example source was changed to hide the typing correction.

Main-suite measurements have 36/36 passing MinZ cells and **zero byte or T-state
changes** in default, size, or Grace. B below is ProgramBytes; T is TotalT.
Artifacts are `/mnt/ssd-510/minz/bench/p28-before` and
`/mnt/ssd-510/minz/bench/p28-after`. Judge and test logs are under
`/mnt/ssd-510/minz/tmp/p28-*`; corpus assemblies and status manifests are in
`/mnt/ssd-510/minz/tmp/p28-corpus-{before,after}`.

| Case | Default B/T before → after | Size B/T before → after | Grace B/T before → after |
|---|---|---|---|
| abs_diff | 27/220,904 → 27/220,904 | 27/220,904 → 27/220,904 | 27/220,904 → 27/220,904 |
| bitwise8reg | 101/872,448 → 101/872,448 | 101/872,448 → 101/872,448 | 101/872,448 → 101/872,448 |
| copy_bytes | 182/301,600 → 182/301,600 | 182/301,600 → 182/301,600 | 182/301,600 → 182/301,600 |
| crc8 | 340/165,616,162 → 340/165,616,162 | 340/165,616,162 → 340/165,616,162 | 326/152,877,602 → 326/152,877,602 |
| fat12_decode | 167/30,336 → 167/30,336 | 167/30,336 → 167/30,336 | 167/30,336 → 167/30,336 |
| gcd | 57/4,953,966 → 57/4,953,966 | 57/4,953,966 → 57/4,953,966 | 51/3,067,013 → 51/3,067,013 |
| memset_fill | 299/18,403,840 → 299/18,403,840 | 299/18,403,840 → 299/18,403,840 | 299/18,403,840 → 299/18,403,840 |
| mul8x8 | 47/1,167,872 → 47/1,167,872 | 47/1,167,872 → 47/1,167,872 | 47/1,167,872 → 47/1,167,872 |
| reverse_loop11 | 561/23,644,160 → 561/23,644,160 | 561/23,644,160 → 561/23,644,160 | 561/23,644,160 → 561/23,644,160 |
| strlen_bytes | 293/19,468,800 → 293/19,468,800 | 293/19,468,800 → 293/19,468,800 | 293/19,468,800 → 293/19,468,800 |
| struct_roundtrip | 71/612,352 → 71/612,352 | 71/612,352 → 71/612,352 | 71/612,352 → 71/612,352 |
| sum_array | 313/7,581,952 → 313/7,581,952 | 313/7,581,952 → 313/7,581,952 | 313/7,581,952 → 313/7,581,952 |
