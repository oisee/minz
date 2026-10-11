# Independent structural rule measurement

Run with Go1.26.8 from minzc:

```
go test ./pkg/pipeline -run '^TestIndependentGraceRules' -count=1 -v
go test ./cmd/minzc -run '^TestHIRProfile' -count=1
```

Set MINZ_PGO_RULE_ARTIFACT_DIR to a scratch output directory to regenerate checksum speed/size evidence. The test uses the unchanged example and saved workload from the scalar stage, recomputes independent host return/memory oracles, checks every raw/target candidate and replays the exact retained winner. Evidence includes per-call target costs, raw profile runs, policies, HIR/lineage and binary bytes/hash. Baseline and constant-if-only match; hoist-only and bundled match. No additional speedup over bundled is claimed.
