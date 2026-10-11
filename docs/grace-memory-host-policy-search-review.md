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

Draft e0521c1f closes both source-review findings: metadata compares exactly
to the independently parsed source; observations preserve Error/Failure and
only clear Steps. Each of the eight candidate records rejects Error/Failure.
Admission and valid retained-SHA altered-body checks passed 7.66 s in the
preserved focused executable3e3547a4e09ab84f70efac00d0003f517a5432d43202ced78438cc3603f3d0fc.
Lifecycle/GC/mutation tests are running, not yet claimed passing. The first
negative fixture was malformed detached provenance; its failed test/executable
are retained and the replacement mutation preserves valid typed HIR.

Focused e052 lifecycle terminal PASS314.10s: forced GC across all48 callbacks,
late original mutation25 PASS42.68s, mutation48 PASS108.18s, cross-policy shared
graph refusal. Final independent source reviews f8a963c8/22b65ee1 and test read
d1022bdc found no remaining source issue. Additional994 owned-input/Size/tie
test still needs execution; full30 search has not run.

Additional994 gates terminal PASS: owned-input mutation/actual short GoalSize
48 callbacks118.45s; missing policies/options preflight4.37s. All failure,
choices and policy binder negatives passed; synthetic primary ties/crossover
are ranking tests only. Preserved executableac88bd7d11cb6f81b2567e4251c051d17be3e44d2ed52a7c0f1e94c257555dd3.
Full30 harnessc11dd4d1 reviewed by both agents (backendfc42b9c8); it independently
collects both policy baselines, exports all eight actual Speed candidates and
labels Size ordering derived. Full30 execution now released; no result yet.


Integration update44739f77: parent copied exactly two reviewed source files,
SHA6357dd5931480ae6baac0daeaadd0c7f19aeb18bed9b337313733f19e50bea77
and a8eed3f8129b0bdbdd0de46b1b9a588c8277a7e52503a96e59a753356e08fcc9.
Full30 actual search terminal PASS83.79s; short control PASS113.95s,
each48callbacks. Preserved executable2955420de33c6ce620f2298b0712ab537f0b66cd7fc540c545d344bff1b55966.
Independent cost65167c16 and backend66824fbc/2842284c audits passed320
observations (full300 plus short20) including typed bindings, proofs, native
bytes/costs, exact state/events and all11 profile counters. Full actual Speed
selects policy1/mask3 1076715T/2100B; short policy1/mask1 566T/2122B.
Derived Size selects Legacy/mask2 1978B in both. Full30 actual GoalSize
was not executed; successful focused994 short actual GoalSize remains separate.
Zero-body short control does not establish dynamic compare activation.

The broader parent gate exposed a test import cycle: same-package frontend
tests import pipeline, whose independently parsed source admission now
imports Nanz, which imports these frontends. The exact independent source
gate remains unchanged. Integration tests are being moved to external test
packages; all assertions remain. Initial failed gate log is preserved.
HIR/MIR/C89/CLI/Nanz passed; showcase31same0new. Full pipeline remains running
on the unchanged reviewed implementation; no overall PASS claimed yet.
