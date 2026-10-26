# Forwarding-only MIR block-parameter cycles

The two-loop RLE fusion checkpoint exposed an unused consumer value carried
through three block parameters in a closed cycle. Ordinary use counts saw each
parameter forwarded to another block and kept all three alive. The emitted Z80
therefore reserved a register and copied an unobserved value on each iteration.

`liveBlockArguments` separates semantic uses from edge forwarding. All instruction
operands, branch/comparison operands and return/error values seed liveness.
Liveness propagates backwards from a live target parameter to the matching
incoming operands. Closed forwarding-only components without a seed disappear.
Entry signatures and implicit DJNZ body counters retain their slots. The ordinary
pass and Grace predicate/action share this analysis. Unknown terminators retain
their edge signatures conservatively.

This is not general cyclic DSE: even an unused instruction's operands are roots.
It does not remove loads, calls, traps or stores; existing DSE has separate effect
rules. After dead edge slots are removed, ordinary DSE may remove newly unused
pure instructions.

## Native checkpoint

The unchanged RLE source and runtime input from the previous checkpoint decode
thirteen bytes with sum 1362. Assembled Z80 measurements include complete linked
code, exclude the sixteen input bytes, and exclude the initial two-byte return
sentinel from stack usage.

| Variant | Code | Linked binary | T-states | Stack below entry |
|---|---:|---:|---:|---:|
| Pull baseline | 311 B | 327 B | 16,201 | 19 B |
| Initial RLE fusion | 73 B | 89 B | 3,576 | 4 B |
| Fusion + forwarding-cycle elimination | 64 B | 80 B | 3,253 | 4 B |

The cleanup saves nine code bytes and 323 T-states on this kernel. Native read
heatmaps still show each source byte read once. The original 16-byte sum kernel
is unchanged at 30 B / 1,586 T-states. These are emulator measurements of concrete
kernels, not universal speed guarantees.

Ten focused cycle fixtures run ordinary/Grace elimination, verify MIR before
and after, execute both versions in the VM and assert a fixed point. They cover
self/multiblock dead cycles, live return/conditional-return/error values, branch
and three-way comparison control, and dead forwarding through `CondRet.ThenArgs`.
Existing DJNZ and forwarded-exit tests also pass. The pointer-strength-reduction
invariant fixture now returns its unrelated parameter as an observable second
result; its original identity-preservation assertion remains.

The 541-test local pipeline run exposed two stale PGO fixture anchors. Fresh
before/after measurement executes all 1,022 u8-length/address-family cases per
compiler against an independent sum oracle, raw MIR2 and assembled Z80. The
ordinary-options guarded module shrinks 248 to 238 bytes; its fast path stays
231+37*n T, while fallback changes from 363+94*n to 326+78*n T. A two-function
contract-optimized fixture trades four extra entry T-states for smaller code:
long_sum(40) changes 1,426 to 1,430 T. Its assembly explains the delta: moving
accumulator preservation to IYH costs eight T, offset by four saved initialization
T. Test coefficients are refreshed in their original compilation contexts; no
production cost evidence is reused across compiler identities. The deep measured
fixture also had a pre-existing 241-T entry anchor: the baseline test fails
because executed entry was already 231 T. Its refreshed anchors now match every
measured row; this historical drift is recorded separately from the new cleanup.

The RLE semantic gate continues to check full traces, exits, aliases and native
memory reads on raw/final MIR2 and Z80. This cleanup affects ordinary MIR
optimization across frontends; generator fusion itself remains opt-in.

## Reproduce

```sh
python3 research/generator-fusion/dead-params/verify.py --out /tmp/minz-mir-cycle-proof
```

The verifier rebuilds the compiler through the RLE gate, compares native costs
against the retained pre-cleanup receipt, runs cycle/DJNZ regressions and records
hashes of the analysis, Grace wiring and tests. `evidence/` retains these outputs.
