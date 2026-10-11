# Constant-call folding must not erase runtime memory or ownership

Base `82da91c7`. Separate production patch only `mir2/consteval.go`; owner of
caller-owned struct buffers adds its metadata guards independently.

Four before-fix independent regressions fail in `before.txt`:

- Function allocates one byte, stores17 and returns its pointer. Caller loads17.
  Folding replaces returned pointer with temporary evaluator heap address13;
  runtime caller then traps on out-of-bounds address13 at heap size13.
- Function writes9 through a constant pointer. Folding retains result9 but
  erases the write, leaving runtime memory73.
- Function reads constant pointer. Folding observes fresh evaluator byte0,
  while runtime caller's byte is73.
- Wrapper calling the writing function has the same erased transitive effect.

A successful VM evaluation does not imply purity. Previously only global
addresses, inline ASM and recursive/fallible/extern boundaries were filtered.
Ordinary heap loads/stores and allocations are real observable runtime behavior.

The new operation-scoped recursive `constantCallPure` admits scalar computation
from explicit arithmetic/conversion/comparison/bit operations and known direct
pure callees. Pointer/noninteger returns, allocation, memory access, ASM, ports,
SMC, stack effects, indirect/conditional/unknown calls and unknown opcodes are
refused. A separate existing global-access guard is retained. No new trust in
CallAttr.IsPure or symbol prefixes; undeclared callee fails closed. Buffer-owner
metadata guards remain independently applicable before this new check.

## Verification

Four meaningful memory/lifetime tests verify behavior after the actual pass,
not just the helper result. Negative admission tests cover unknown opcodes,
unknown call, load/store/allocation/ASM/ports/SMC/indirect; arithmetic remains
eligible. Existing pure Double/chainedCalls/constant-call suite remains passing.

Whole Nanz source regression keeps all functions unchanged, executes write,
read and transitive wrapper entries at target RAM24576 with initial73. Raw/final
VM and assembled native ordinary/Grace Speed/Grace Size return exact values,
retain writes and check all other RAM, executable image and balanced PC/SP.
This is a test fixture, not new exact-corpus coverage. Native external observe
declaration is intentionally unused and is not executed.

Read-only ABI audit packet `5af4551b` records related consumer findings. This
patch deliberately leaves ordinary MIR-call arity validation to a separate
contract decision; it does not weaken any profile or ownership boundary.

Terminal full MIR2 package PASS84.665s; final focused purity +pure arithmetic
PASS0.010s (after additional host/no-body denial controls); whole-source native
gate PASS0.015s. Metadata-owner struct-return hunks untouched.
