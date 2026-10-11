Read-only native provenance review, isolated from shared memory-host owners. PLAN.md contains the recommended contract. The diagnostic confirms two currently allowed unsafe native accesses at checkpoint058458cb while VM proof does not authorize their numeric MIR pointer. It records a runner-layer counterexample; no claim that a forged complete public oracle was accepted.

Reproduce from this worktree's minzc directory:
GOCACHE=/home/alice/dev/minz-artifacts/grace-cache/go /home/alice/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin/go test ./pkg/pipeline -run '^TestMemoryHostCheckpointNumericGlobalPermissionGap$' -count=1 -v

No existing production file was edited; only this permanent checkpoint and diagnostic new test are owned here. General tests were not rerun. Future refusal implementation must replace the diagnostic expected-permission assertions rather than preserve unsafe behavior as a compatibility requirement.

STACK-SPILL-REVIEW.md and numeric-stack-spill-gap.txt extend the same runner-layer diagnosis to arbitrary numeric stack/spill reads and writes. Reproduce only that test with -run ^TestMemoryHostCheckpointNumericStackSpillPermissionGap$. No general tests are required for this evidence.
