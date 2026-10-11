# Native wide shift placement and migration audit

Integrated d0383543 fixes constant left-shift of a dword placed in DE/BC.
Previously only the main half was copied through HL; a legal pinned DE
source a=0xffff, a<<1 returned0x0000fffe instead of0x0001fffe. Ordinary
automatic allocation stages through memory/HL and masked this path.

Non-HL pairs now use an explicit four-byte carry chain across main and
shadow banks without touching unrelated HL. Root source-backed legal
placements and1470 forced native cases PASS: pipeline0.031s, MIR2 2.764s.
All nine source/ASM/BIN payload SHA256 values verified. Original before
logs preserved; no automatic-allocation failure claim is substituted.

Root rechecked all53 archived LIR file contents against the original
manifest and verified absence of the live LIR package/exact Go imports.
The effect-safe load/store fusion migration still lacks proof and is now
being implemented separately against real FAT12 helpers. No full transfer
completion is claimed from the package deletion alone.

Original14 error code/control semantics is being repaired in parallel.
Full pipeline8bf remains live at47minutes; no terminal claim yet.
Goal active; current bounded corpus remains48paths.
