# Independent call-contract consumer audit

Pinned root: `82da91c7d14830cc674e93e4e0592088eea04cc1`.
Read-only production audit; only report/probe artifacts are committed.
Struct-return implementation read separately from the owner's uncommitted
`grace-workspaces/struct-return-lifetime`, at `81de9abc` plus working edits.
Those observations are implementation advice, not an integrated-code claim.

New fields there: `mir2.StructReturnBuffer{ReturnIndex,ParamIndex,Ty *StructTy}`;
`Contract.StructReturnBuffers`, `Inst.StructReturnBuffers`; HIR signature derived
metadata. `LowerStructReturnBuffers` appends pointer ABI parameters, supplies
caller OpAlloca buffers, snapshots original returned object then copies bytes;
VM public Call/CallFunc/CallWithOutcome binds source arguments to hidden buffers.

## Confirmed probes on the pinned root

Run from minzc: `go run <packet>/probe.go.txt` after copying to a .go pathname.
Exact runnable program and stdout retained; no compiler expectation is the oracle.

1. MIR Verify accepts ordinary direct call with missing argument: `<nil>`.
   `verify.go:40–66` validates fallible call extras but no ordinary call arity;
   `verifyFunc` checks SSA dominance, not callee ABI. New buffer metadata needs
   independent validation: unique contiguous hidden ParamIndex, valid return
   index and exact object layout, pointer width/class, direct-call metadata equal
   callee contract and complete physical argument count. Indirect signatures
   require the same derived buffer contract; absent authority must fail closed.
2. `ConstantCallElim` can discard an ordinary pointer write: store_byte(const0)
   changes runtime heap[0] to9 and returns9, yet pass reports changed=true and
   substitutes const9. `consteval.go:85–104` checks globals/ASM/recursion/fallible,
   not ordinary stores or caller-visible pointer effects.
3. More decisive for return ownership: local() allocates one byte, stores17,
   returns pointer; use_local calls then loads it. Before pass returns17.
   `ConstantCallElim` changes it, then a new runtime VM traps reading address44
   at heap size44. A temporary evaluator's pointer must never become a runtime
   constant. This is a genuine existing MIR optimization failure, not a new ABI
   test failure. Buffer-bearing callees/call sites need immediate exclusion;
   transitive pointer/allocation/store effects require a separate complete fix.

## Consumer findings sent directly to implementation owner

| Consumer | Concrete current reference | Required handling |
|---|---|---|
| Const-call | `mir2/consteval.go:35,52,99` | Inst.Args includes hidden args, but public vm.Call now expects source count. Keep distinction explicit; exclude buffer ownership/effects, never fold pointers from evaluator heap. |
| Public native asserts | `pipeline/pipeline.go:933–972` buildAssertBootstrap | Iterates source Assert.Args only: hidden result pointer remains uninitialized. Allocate explicit owned native result storage or reject this entry shape clearly; preserve public count/identity. |
| Public measured workload | `pipeline/profile_workload.go:334–361` | Exact MIR Params/HIR Params count check currently fails closed for aggregate entry. Preserve public source parameter count; any native adapter must bind hidden storage privately, compare all semantic return bytes with pointer relocation declared. |
| Public controlled VM | `hir/profile_run.go:98–121` | Source-argument count is correct. Public VM autoBind may allocate result storage per call; memory budgets and exact-access/snapshot authority must cover those allocations explicitly. |
| Inlining | `mir2/inline.go:86,169,237` | Exclude Contract.StructReturnBuffers unless lifetime/copy ownership remapping is proven; param loop currently maps all physical params. remapInst is shallow copy; mutable metadata slices must not escape authority assumptions. |
| Tail calls | `mir2/z80codegen.go:1078–1090`, `grace_runner.go:1235–1252` | Current local-frame check usually blocks buffered calls because of OpAlloca. Add explicit buffer lifetime guard before any future forwarded/static-buffer optimization; tail JP must not expose caller-local storage after frame teardown. |
| Conditional call clone | `mir2/grace_runner.go:1779–1792` | Fresh OpCallCond only copies Sym/Args/Ty/Cls/Cond, drops CallAttr. Existing predicate excludes FallibleCY and requires single void call; buffer/extra/signature metadata should also be explicitly ineligible or preserved. |
| Dump/artifact | `mir2/dump.go:50,182–196` | Dump currently omits buffer descriptors. Reflective full-MIR JSON fingerprints include new fields automatically; text dump needs metadata to make saved artifacts reviewable/coherent. |
| Extern identity | `hir/memory_host_workload.go:397–410` | Compares attrs/params/returns/clobbers; add buffer descriptor equality if extern buffered ABI is ever admitted. Current extern aggregate refusal remains safe. |
| HIR signature | `hir/captures.go:106–125` | Current comparison only Params/Returns. New derived buffer list must compare indexes and layout; pointer address alone is insufficient semantic callable identity. |

Indirect return lowering observed in owner's draft `struct_return.go:44–47`
creates only ReturnIndex0 from Inst.Ty. If indirect multi-return aggregate slots
are accepted later, derive all slots from signature/ExtraRetTys, or explicitly
reject unsupported shape. Direct-call slots already derive callee contract.

Generic call clobber/ABI optimizer `mir2/contracts.go:48,178,215` intentionally
works on physical ordered Contract.Params, including appended hidden parameters;
that physical participation is correct. Public/source argument consumers must
not use this physical list as a public signature accidentally.

Existing CY protection remains: verifier checks fallible ABI/output shape;
consteval and trivial inline exclude fallible; tail-call action excludes caller
and call FallibleCY. Consumed callback agent explicitly rejects fallible actual
targets because CallSignature currently lacks CY; preserve that refusal until
signature has independent CY authority. Do not infer trust from prefix or raw
function address.

## Fingerprint and ownership boundary

`hir/profile_compat.go:56–86` reflects the semantic HIR graph, so new public
signature fields automatically enter source compatibility. Candidate routines
`pipeline/profile_modes.go:179,229` and `profile_modes_workloads.go:195,236`
serialize complete MIR and allocation as well as text/ASM checks; new metadata
must remain in those complete snapshots. Root's strong node retention observes
new slice/pointer nodes through reflection. That does not replace explicit
verifier validation of a forged descriptor's ABI or object layout.

No claim that draft schema or all new consumers are fixed. Owners received
findings and exact probes during implementation. No production edit and no
full expensive suite restart were required for this bounded audit.
