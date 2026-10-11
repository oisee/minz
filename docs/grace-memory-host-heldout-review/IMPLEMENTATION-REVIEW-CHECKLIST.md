# Private fixed evaluator review gate

Implementation review is pending. No evaluator code or new target execution was performed by this branch.

* Admission precedes any baseline execution: exact known source bytes and logical unit, frozen training decision and its context/hash, four preregistered byte-identical workloads, validated typed policy and finite masks.
* The API accepts owned source/workload/decision inputs; it does not accept arbitrary transformed factories, assembly, external semantic oracles or supplied evaluation profiles. Every expected original HIR is independently parsed from the frozen source.
* Every source callback passes complete semantic metadata, body/ABI and function/path provenance checks against the independent expected original before transformation. Capture graph roots before destructive rewrite or compile; strong references span all calls and errors.
* Actual candidate callbacks apply only the two finite known transformations and invoke the existing independent aggregate precompile proof. Proof records must match across callback invocations. Ordinary functions/loops and origin maps are preserved.
* Existing policy binder is unchanged: supporting1/0 original binds trained1/3;0/0 original binds trained0/2. Primary report control remains0/0. No cross-policy native evidence is relabelled.
* Four runs per workload are explicit:0/0 report control,1/0 proof support,1/3 trained Speed,0/2 trained Size. No candidate ranking or reselection occurs on these evaluation inputs.
* Full original/variant raw semantics and all closed-host target observations agree except execution counts. Any typed denial or infrastructure failure rejects the complete evaluation; retain diagnostics, never report a best surviving subset.
* Meaningful focused negatives cover immutable input mutation, malformed source/unit, tuple drift, unrelated body mutation retaining SHA, shared/reused source nodes, storage overlap, wrong same-policy baseline and missing/failed evidence. Real policy activation controls are separate from API-only tiny fixtures.
* Native runs remain blocked until frozen source inventory, two independent reviews and root release. Save exact executable, complete envelopes, precompile records, source/workload hashes, outputs and error history before assertions.
