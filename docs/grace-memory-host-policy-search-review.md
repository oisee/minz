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
