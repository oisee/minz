# C89: practical good-enough coverage

Alice's goal: cover C89 reasonably well with C tests and verify that aggregate
input/return idioms decompose and optimize into multiple arguments/results.
Base main: 548ee6ce. Work branch: feat/c89-good-enough.

## Completion criteria

1. Publish a language-feature matrix based on executable C cases, with supported,
   explicitly rejected and still-open states. Parsing success and historical
   assembly coverage are not semantic coverage.
2. Cover target integer conversions/promotions, expressions and evaluation,
   control flow, declarations/scopes/linkage, pointers/arrays, aggregates,
   initialization and qualifiers needed by embedded/system C programs.
   Float/64-bit values and other target limits must be documented and explicitly
   rejected where unavailable; do not claim full ISO conformance.
3. Use checked-in C inputs and independent expectations. Run raw/final MIR VM
   and actual assembled Z80; check results, memory/effect order and native PC/SP.
   Derive expectations from C semantics and the documented Z80 ABI, avoiding
   host-int-width differences, unspecified evaluation order and undefined C.
4. Add explicit positive and negative cases for small-struct return and by-value
   input decomposition. Eligible scalar fields become multiple return values
   and argument slots, with real MIR/ASM evidence and measured transfer cost.
   Mutations, side effects, branching, forwarding and live caller values retain
   their meaning. Address escape, volatile/overlapping storage and indirect ABI
   boundaries must remain correct, using memory when necessary.
5. Verify struct by-value copying and struct-return lifetime independently from
   the optimization decision. A passing numerical test alone does not establish
   scalar decomposition; a tuple-looking dump alone does not establish correctness.
6. Fix reproduced errors and preserve minimal C regression cases. Add broader
   generated/heldout cases where they check distinct behavior. Keep practical PR
   tests parallel; reserve genuinely deep campaigns for release/nightly/manual.
7. Keep the real FAT12 example and mixed Nanz/C path working. Required strict CI
   passes final head against current main; merge examples, tests and documentation
   into main and reproduce representative commands there.

The first step is the aggregate ABI audit requested by Alice, then expand the
feature matrix from actual gaps. Coverage percentages must name their denominator.
