# Repeated normal-global load reuse: read-only plan

Base: 86bfdd16ad0a88549bdc269f9a484420fe7f356a. No compiler edits or measurements in this checkpoint.

## Observed source and current architecture

The unchanged SelfLanz `cur` first loads `pos` and `src_len` in entry to test the boundary. Its sole in-bounds successor loads `src_buf`, then independently constructs `AddrOf(pos)` and loads `pos` again, before calling `peek`. The out-of-bounds successor returns zero. The saved final MIR contains entry r49 and repeated r57, both u16, but registers are illustrative rather than matcher identities.

Grace `redundant-load` currently uses `valueKey` in grace_runner.go. AddrOf falls back to register identity rather than global symbol identity. `canForwardLoad` excludes effects anywhere in both predecessor and successor; the later peek therefore blocks forwarding. Local pointer CSE deduplicates ptr_add expressions, not loaded memory values. DSE deliberately treats arbitrary OpLoad as potentially volatile. Neither generic rule should be weakened for this slice.

## Proposed isolated experiment after review

A private MIR helper, default off and invoked only by an isolated test harness, accepts verified preallocation SSA. Restrict the first experiment to an entry conditional branch and a non-entry successor with exactly that one incoming predecessor, without backedges or block-parameter ambiguity. Match two direct OpAddrOf definitions naming the same unique module Global: StorageNormal, At nil, exact u16 object and exact u16 load type. No numeric pointers, pointer parameters, offsets, fields, hardware mappings, SMC/baked storage or resource loads are eligible.

Scan the actual ordered interval after the first load through the predecessor suffix and successor prefix ending immediately before the repeated load. Permit pure arithmetic/address computation and independently proven normal-global loads only. Any call, indirect call, store (even to another symbol), assembly, patch/SMC, stack allocation, unknown opcode or potentially volatile read rejects the candidate. A call after the repeated load is outside this interval and cannot invalidate the prior memory value. Do not use function-wide purity or CallAttrs to waive barriers.

Replace the second load with an OpMove whose source is the first loaded register, preserving the second definition, type, class constraints and source location. This keeps all existing uses—including call arguments, asm inputs, extra operands and terminators—pointing at the same destination, avoiding partial operand remapping. Remove its dead AddrOf only with the existing canonical-use-aware DSE. Verify before and after; inspect hard classes and contract/allocation consequences. A later optional PropagateCopies must use canonical complete operands and retain verification. This is a bounded diamond, not a general global-value-numbering algorithm.

The eventual insertion point must precede OptimizeContracts and PreallocCoalesce; the initial experiment can invoke the helper immediately after raw lowering followed by verification. Final non-SSA allocation MIR is never input to this proof. Native memory roots must be regenerated from the actual transformed MIR and allocation, not inherited from the source load.

## Semantic scope

Ordinary compiler-owned global memory is distinct from declared host resource RAM. Eliminating an internal global read changes raw VM steps and internal memory access counts; these are not the source semantic oracle. The closed host contract keeps declared resources separate from globals and provides no callback that can mutate globals between the two loads. Final results, global/region state and ordered peek/poke events must remain exact. This experiment makes no asynchronous interrupt or external volatile-memory claim and has no public API/CLI default activation.

## Required gates before measurement

Static verified MIR positives: both branch outcomes and unsigned boundary cases, same-symbol distinct AddrOf definitions, late call after repeated load. Refusals: intervening same/other-symbol store, direct/indirect call, asm, patch, opaque or numeric read, AT/baked global, wrong width/type, multiple predecessor, loop backedge, non-dominating load and duplicate definitions. A meaningful callback that changes pos between loads must retain the second read. Keep source and transformed MIR snapshots distinct.

After independent source review, compile actual source and candidate through the identical closed baseline pipeline, comparing full original host oracle and ordered resources. Measure costs rather than assuming that removing a u16 load saves 16 T: longer liveness and allocation can change spills and calling convention. Preserve ASM/BIN, all typed native roots, allocation and MIR bytes, contexts and exact costs. Existing InlineTrivial and generic redundant-load behavior remain unchanged.

## Separate immediate priority

The four previously reviewed persistent corpora are being refreshed artifact-only on frozen production executable dba34. These eight CLI runs do not implement or measure this proposal. Their current costs determine later opportunity ranking.
