Independent unsigned preserve-pair compare review begins with a private default-off option. Source and all closed host/native permissions stay unchanged. The arithmetic model checks 786513 C/NC/Z boundary paths, including all equal-high low-byte pairs, every high-byte pair with opposite low ordering, incoming carry/zero combinations and 0/255/256/32767/32768/65535 edges.

`LD A,lhs.hi; CP rhs.hi; JR NZ,done; LD A,lhs.lo; CP rhs.lo` preserves both pairs. Carry encodes unsigned lhs<rhs, so C implements Ult and NC implements Uge in original operand order; Z encodes whole-word equality. Other F bits are not equivalent to word SBC and cannot be consumed as signed comparison results. High-different costs 20 T, high-equal 23 T, six bytes. This is sequence timing, not measured program improvement. Original HL/DE SBC can cost only19 T; select only where actual staging/destructive pair preservation would otherwise be needed.

Review gates before native full30:

- Exact U16 operands already materialized in distinct HL/DE/BC; no unresolved constants, physOverride alias mismatch, indexed/byte/spill/promotion form. Preserving a physical pair must not claim its value equals an unmaterialized logical constant.
- Ult/Uge ordinary flag result only, direct consumer branch/conditional return supported; maintain original orientation and reject two-test/signed/synthetic predicates.
- A dead over actual function successor uses and pending accumulator, not only this block. Existing `regsLiveAfterInst`, pending state and generic materialization semantics must be respected. Live old flag/acc values must fall back, without changing fallback code bytes.
- Only A cache becomes invalid. HL/DE/BC aliases and pending logical flags remain consistent; pendingFlagReg becomes current result. Scratch preservation wrapper must not duplicate saves or inadvertently restore old F.
- A local forward branch must be bound to emitted instruction sequence; bytelex incoming C/Z does not matter. No native memory authority change is needed for register-only operations.
- Forced allocations exercise operand orders, both conditions, high and low boundaries, flags, future use of both pairs, live A, pending state and fallback classes. Existing current production original must match exact binary/assembly/cost/state before accepting a full native experiment.

Native runs, after final frozen source review, belong to goto_artifacts only: original versus compare, plus separate reuse+compare. Source model checks complete results/globals/resources/events; source profiles must be recollected for the new executable context. No CPU execution has occurred in this review.
