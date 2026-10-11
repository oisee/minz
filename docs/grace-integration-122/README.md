# Supported package compile gate after profile admission repair

Root b0be78e3 compiled every supported pkg/cmd/internal/tests package with
`go test ./pkg/... ./cmd/... ./internal/... ./tests/... -run '^$' -count=1`: PASS.
This is build-only; no runtime test correctness inferred. LIR remains absent
from the live compiler; historical archived research is retained unchanged.

Runtime gate after integration of error semantics, sequential-byte fusion
and wide parallel-copy fixes still required. Existing current49 bounded
VM/native corpus paths and three frozen heldout families remain their
explicit scoped evidence, not a declaration all194 paths execute correctly.

Two discovered live correctness gaps remain under repair: error code/CY
propagation and u32 loop parallel copies losing shadow half. Byte-word
fusion migration must preserve callback instruction identity and byte fault
prefix before admission. Goal active.

Root PFCCO checked-model refresh PASS0.012s:600 deterministic forest
tasks, independent exhaustive optimum over three classes (up to729
assignments), plus native adapter costs including real CALL/RET/liveBC
saves and infeasible refusals. Dropping21T of saves intentionally changes
the chosen class to a worse real-cost result. Diamond/chain regressions
pass. This is bounded model evidence, not arbitrary graph/global native
optimality.
