# Policy search: parent review of draft4af821fe

New source is isolated in `feat/memory-host-policy-search`; it is not integrated.
The proposed search binds independently parsed known source bytes and both
whole policy association inputs before entering the caller factory. The
existing fixed-policy selector implementations remain unchanged.

The parent read confirmed the 24 callbacks per policy lifecycle and common
strong graph roots. Immutable originals are rechecked before and after caller
reentry and before returning. Both input envelopes are decoded/preflighted
before the first source callback; each inner selector still performs fresh
replay and exact candidate precompile/native checks. Stable Legacy-first
ordering compares the requested primary cost only.

Two findings must be closed before measurement:

- BuildProfileCompatibility deliberately removes assertions, sandboxes,
  warning and assertion accounting metadata. Exact unchanged-source admission
  needs a separate comparison of this metadata to the independently parsed
  reference, with negative tests retaining the original source SHA.
- arenaSameObservations removes nonempty Error text. The new search contract
  permits only native Steps/cost to differ. Use a separate exact observation
  comparison for search, preserving Error and structured Failure identity;
  retain the old fixed-policy helper for its existing callers.

Independent full-function body/ABI/origin hashes already address the earlier
retained-source-SHA altered-AST concern. Meaningful tests still need to prove
that gate rejects changes in unrelated functions, late mutations across
policy boundaries and shared graphs while accepting independent fresh graphs
under forced GC. No new CPU result or completion claim follows from this read.
