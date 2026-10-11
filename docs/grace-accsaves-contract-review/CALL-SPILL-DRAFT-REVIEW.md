# First native caller-spill certificate draft review

Read the new permanent call-spill-native-certificate worktree, based on c0b74f6a. This is a draft review; no native execution or owner edits by this reviewer.

Emitter identity correctly finds the actual call instruction pointer in the original block, rather than using reordered emission index. Save/restore markers are distinct stages and exact finite byte/word opcode sequences can be independently derived. Runtime E3 uses the actual read-low/read-high/write-high/write-low order, verifies reserved initialized HL slots and live return-frame exclusion before effects, then checks unchanged SP and exchanged HL. E3 remains outside the generic frame permission gate. These are sound narrow design choices.

Admission issues sent to the backend: derive CallID independently from actual module function/block/call instruction, rather than accepting any nonempty string; validate the home is a live-across value and not a call result, using actual allocation/generated-home proof; use the exact native spill width, rather than accepting all source types wider than eight bits as a two-byte home. Existence of a register in regInfo alone is not sufficient authority for an arbitrary generated spill home.

Runtime records should retain the actual saved snapshot word and reject changed slot contents at restore entry. Matching SP, call depth, symbol and initialized HL role alone cannot distinguish a corrupted/replaced word. Pair identity must associate both stages with the same actual callsite while allowing recursive repetitions at different activation depths. Require no unretired records or active stages at a completed top-level invocation. The resolver must reject overlapping stage PCs/result-pickup PCs and ambiguous markers; intended sidecar spill load/store coannotation is allowed only with equal independently derived symbol/width/direction.

These findings are pending draft hardening, not claims of a final exploit against the private compiler-owned runner. Preserve the actual previous EX(SP),HL PGO refusal history and require focused meaningful negative controls before the next actual closed-program execution.
