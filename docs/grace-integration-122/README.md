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
