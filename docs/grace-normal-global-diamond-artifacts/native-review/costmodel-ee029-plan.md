# Bounded dominating global-load reuse

Read-only design at 86bfdd16; no parent, source or compiler edits, no CPU run.
Actual cur uses pos:u16 in its entry comparison, then rereads pos on the inbounds
successor before peek(src_buf+pos). No store, callback or opaque operation lies
between these reads. Forwarding the loaded value might save a global read, but
longer lifetime, edge copies and register allocation can offset that benefit.
Actual caller measurements decide; do not subtract 20 T times calls as a result.

Existing MIR Grace redundant-load logic is not directly applicable. Its address
valueKey uses rN for AddrOf, so separate addresses of the same global are not
identified; canForwardLoad rejects all effects in both entire blocks, including
the later peek after the redundant read. Do not globally relax those guards or
add generic alias/purity assumptions for arbitrary externs.

Smallest first prototype: private opt-in direct-symbol u16 load forwarding for
the exact acyclic entry-to-inbounds edge. Require verified SSA, a known normal
module global of exact width16 with no absolute AT/device/SMC/phantom storage,
matching exact zero-offset OpAddrOf+OpLoad in pred and succ, same load type,
and unique predecessor of succ. First value must dominate successor. Reject
loops/backedges, merge with another predecessor, unreachable/ambiguous CFG,
unknown pointer roots, mixed widths and in-place source-register mutation.

Track effects along the interval from dominating read to its exit and from
successor entry to redundant read only. Whitelist typed pure arithmetic,
comparison, conversions and known normal-global reads; every store (even a
seemingly different pointer), call/indirectcall, asm, patch, stack/memory-opaque
operation or unknown opcode invalidates. Never infer arbitrary extern purity.
A later call after the consumed load need not invalidate its earlier reuse,
but must remain in its original position and keep its full resource semantics.
No hoist crosses a possibly trapping earlier resource read. Numeric/resource
pointers and volatile/device absolute globals are out of this first contract.

Mechanically supply the dominating value as a successor block argument, remove
only the redundant load, and remap ALL canonical uses (Src, Args, Asm inputs,
terminators) with correct type/class. Update every incoming pred edge to succ;
unique predecessor doesn't mean a single branch edge. Retain required AddrOf
uses until canonical dead-code/liveness cleanup. Verify MIR afterwards, roll
back on refusal. Do not introduce a stale codegen global cache or shadow source
bindings. Native origin descriptors are rebuilt from final MIR, not copied from
a removed load; remaining source HIR/site provenance stays unchanged.

Memory equivalence scope: internal compiler-owned global read counts may reduce
under this explicit normal-global optimization. Host resource events (peek/poke
byte order and failure prefix), globals/resource values and call behavior must
remain identical. Do not claim exact internal-global read traffic. MIR/VM gas
steps and instrumented profile steps can change; compare their meanings without
calling reduced steps a semantic memory-denial difference. No HIR loop rewrite
or postcompile lineage certificate is implied. A new executable collects fresh
profiles; old payloads must refuse instead of having IDs patched.

Meaningful gates: safe exact cur-shaped diamond vs store-through-same/unknown
pointer, intervening call/asm, AT/device global, mixed width, two incoming preds,
loop reentry, same-global different source register, duplicate SSA and target
used in Args/Asm. Raw original/transformed VM should observe store/call side
effects, not merely compare node counts. Actual Z80 needs live value across
comparison/branch and subsequent address arithmetic, normal return/bounds0,
resource typed-denial prefix, full original30 host state/events and rebuilt
native descriptors. Profitability is measured for unchanged source, optional
old/new both-transform candidate; no expanded default optimizer before proof.

Priority recommendation after current publication: first refresh the four
existing workload corpora with the new production CLI (eight goal runs, same
inputs/host model) as a bounded artifact-only exercise. Arena's full30 increment
changed from3099 to159 T, so old corpus selections cannot establish new-backend
costs. This provides current crossover data while the reuse proof is designed.
A new reusable optimizer comes second after an isolated meaningful gain and
alias/effect negative gates; the existing correct closed runtime is unchanged.
