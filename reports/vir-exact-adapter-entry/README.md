# Adapter insertion uses the exact function entry

Previous insertion targeted the first non-local assembly label, even if it named an unrelated function; a missing target label silently returned an unadapted body. Permanent native regression preserves an unrelated RET before target and executes target on all256 byte inputs. Old code modified other instead of target and failed. New code requires exactly one f.Name entry before inserting nonempty moves; missing/duplicate entries reject, including comments/whitespace handling.

Broad actual-Z3 adapter/word/indexed/island/codegen/assert family PASS7.733s. Existing global word ABI fallback still passes its65536-domain native test. This is emitted-entry correctness, not proof of arbitrary assemblies or all language modes.

Also removed unreachable dual-mode winner code: a successful constrained solve already returns before standalone solve, so instruction-count plus one per changed parameter was never a live cost comparison. Current strategy remains constrained-first; standalone+adapter only after constrained failure. Log parameter count is not instruction/T/byte cost. No performance policy change, Speed penalty, or new measured benefit claimed. Full production cbf01110/research7bfe4476 remain live earlier checkpoints, not restarted.
