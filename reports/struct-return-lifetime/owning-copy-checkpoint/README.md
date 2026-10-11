Intermediate ABI correction after buffered-v2 (6c032fe9).

Frozen loop/copy source: 56a17420. Before correction raw VM retained_loop(0,83)=2, expected0: a held local pointed into the reused call-site result slot. Struct locals now own fixed frame storage and copy bytes on initialization/assignment; inline nested fields copy their object bytes rather than pointer payload. Source field mutation still addresses the original local object.

Cross-result return copies snapshot ALL slots before ANY destination store. Typed VM indirect dispatch checks actual target parameter/return types and buffer layouts, rejects unsupported CY-error target before entry, and captures extra returns. Address-taken functions have a derived FixedIndirectABI attribute: canonical ABI parameters are moved to their allocated homes once at function entry; direct calls to them use the same canonical convention. Allocation may spill long-lived inputs; this is an explicit bridge, not a HasAsm disguise or an assumed successful pin.

Tests: unchanged lifetime30 native calls; extended nested/recursive/indirect/held-loop/independent-copy123 native calls over default/GraceSpeed/GraceSize, matching raw/finalVM, full64KiB comparison with only observed stack and declared mutable bytes. Native code remains unchanged and PC/SP restored. Targeted metadata/publicVM tests PASS.

Still required before general final ABI claim: independent overlapping multi-result copy oracle, nonoverlapping canonical argument slots for multiple pointer/word arguments, checked-table bridge admission, complete native public entry/multi-return evidence, original Widemath combined-root replay and composite metadata clone ownership. This checkpoint is pushed to preserve work and make review concrete.
