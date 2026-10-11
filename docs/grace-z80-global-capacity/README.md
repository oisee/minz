# Native Z80 global memory admission

Z80Codegen rejects a single oversized global, oversized initializer or sum of global storage above65536bytes before producing ASM. Previously story131072 produced393272characters of assembly without error. The check follows symbol validation, preserving malformed-symbol diagnostics.

This is a necessary data-only check, not complete code/data/layout capacity proof: code, alignment and runtime storage reduce usable space further. HIR and MIR VM are unchanged; host-sized programs must select an appropriate backend.

The actual ZVM source documents mzv -H and contains a131072byte story buffer plus host file/UTF8/input externs. Fixing nativeu32 memory/caller behavior remains required and is proceeding independently; the large interpreter cannot silently become a flat64KiB Z80 binary by changing its source allocation.

FullMIR2 after change PASS6.748s; focused oversized/ordinary global checks PASS. Historical artifacts stay unchanged.
