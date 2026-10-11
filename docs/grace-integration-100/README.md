# Ownership HIR survives detached-node collection

Production fix `c0a7e49f`: selectors retain actual copied pointer/map/slice references in their operation-local ownership map. Retaining Module alone is insufficient when compilation replaces its Body or other graph fields: a uintptr-only identity record does not keep old nodes live and can falsely identify a later independent allocation as shared storage.

Canonical immutable MIR type exemptions are unchanged. The check still rejects shared AST nodes and backing storage; full source/HIR/ABI identity and mutation checks remain in force. No global cache is introduced. Original graphs remain retained for the duration of the selector, so memory use can grow with graph size and candidate count. The previous implementation already retained modules; this additionally preserves detached storage.

Root regressions PASS0.011s: a weak reference to a detached Body survives GC while ownership is tracked; true reuse is rejected; a distinct graph passes; shared zero-length slice with retained capacity is rejected. Existing memory-host GC/cap check passes. Root compiler-mode and joint selector admission/late mutation/shared HIR/forged counts/deterministic selection tests PASS2.936s. This is not a full pipeline run.

The original packed FAT automatic selector is rerun separately after controlled unreachable-extern admission. No winner or heldout gain is claimed here.
