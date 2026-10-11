# Word spill copy review

Read-only follow-up on isolated `/tmp/minz-forward-copy`. Production fixes are stable in z80codegen_move.go (16-bit emitMov spill-to-spill) and z80codegen_copy.go (emitSingleCopy spill-to-spill).

The failing intern caller computes the checksum data pointer in HL and then copies the spilled seed into a second spilled argument. Previously that copy loaded the seed into HL and left it there. The new PUSH HL / copy / POP HL sequence restores the pointer and balances the stack. The production direct OpMove path uses ordinary word LD; these instructions preserve flags and A. Restoring the original HL means its preexisting holdsPhys aliases remain correct, so removing invalidate(HL) is appropriate for this scratch-only change.

emitSingleCopy uses loadSpill16/storeSpill16 helpers. Traditional source/destination word LD preserves A and flags. TSMC reload uses LD HL,immediate and also preserves them. TSMC destination patches immediate bytes through A and calls invalidate(A). The completed fix detects that destination, saves AF before the HL scratch save, then restores HL and AF in reverse order and conservatively invalidates A aliases. Three real machine-code TSMC regressions verify the patched reload value, live A and pending parallel A→E source, live HL, carry and SP. Traditional production OpMove retains only the HL save and adds no AF cost.

Parallel copies still preserve source ordering and resolve cycles before delegating to emitSingleCopy. A spill-to-spill temporary HL must not overwrite another pending HL source or a completed HL destination; saving/restoring HL addresses both. The new regression explicitly puts a spill-copy before a pending HL→DE copy and checks the latter receives the original pointer. Existing mixed pair/half and spill parallel-copy tests are included in the full MIR2 gate. The change preserves rather than consumes HL, so no new copy-cycle edge is introduced.

Nine real-machine regressions cover direct OpMove and parallel spill copies with values1/ABCD/FFFF, observable liveHL, A, carry, SP and pending HL→DE source. Baseline typed intern caller30calls also passes return/global/resource/exacttraffic checks, with final bump559 and hits15. Original failed artifacts remain separate from fixed results.


Final gates: full HIR7.356s and MIR2 3.811s passed before TSMC extension; focused oldcopy/weighted/call pipeline24.088s passed. After the material TSMC extension full MIR2 passed4.208s, covering the new tests and existing parallel cycles. The actual intern path uses traditional named spills; the TSMC branch is selected only for a matching TSMC destination.
