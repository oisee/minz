# Final matcher read a149a48f

The new DE/BC rule now requires canonical function-wide countRegUses==1 and a
known global with at least two declared bytes. This covers direct successor
uses, indirect-call operands, asm input bindings and terminators. Missing
function context refuses new selection. Existing HL selection and its assembly
comment are unchanged; no legacy matcher broadening is introduced.

The emitter uses the same MIR-rooted native memory descriptor (offset0, width2)
and invalidates only the destination pair plus its byte aliases. The existing
invalidate helper correctly kills DE/D/E or BC/B/C cache entries; the direct
instruction preserves A, flags and HL. No remaining concrete source-level
legality issue was found in this bounded read. No CPU was executed here.

Focused test review requested two improvements before evidence freeze: exempt
only actual opcode-fetch bytes from instruction read-count checking, not the
whole binary (globals are binary data); make the successor byte load's value
observable in an actual result rather than only asserting retained address
materialization. Inline asm input use is handled canonically and merits a
specific refusal fixture if practical. These strengthen evidence without
changing production scope or repeating full native caller runs.
