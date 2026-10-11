Follow-up after cfa18d68: canonical return slots use the same deterministic distinct register allocator as parameters. Native indirect pickup uses all semantic return slots. Earlier position>=2 General could collide; physical capacity refusal remains explicit.

TestStructReturnCrossSlotAliasSnapshot uses a declared two-object MIR function and independently initialized 64KiB memory. Three overlapping source/destination arrangements include addressffff→0000 wrap. Both VM and actual assembled Z80 retain the pre-copy snapshot across all result slots, return both HL/DE buffer addresses, restore PC/SP, preserve code and all memory outside exact destination bytes/spill slots/observed stack. The all-RAM native runner explicitly disables Spectrum ROM protection; a prior test with protected low RAM was a harness failure, not a backend miscompile.

Historical TestProductionCallFixRound1/indirect.nanz no longer expects unsupported: its original assert go(3,1000,9)=5667 executes through the ordinary assembled-native assert API, covering live scalar/word caller values. All existing call-fix fixtures pass. No original expectations changed.

This does not yet prove every typed multi-return call/external bootstrap shape or whole-language aggregate ABI; original Widemath combined-root replay and metadata lifetime review remain.
