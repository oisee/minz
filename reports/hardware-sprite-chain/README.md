# Patchable sprite chains: implementation checkpoint

October 13, 2026. Work in progress on `feat/hw-patchable-sprites`; not yet in main.
The goal still requires full Spectrum execution, animation, expanded masks and
refusal coverage, comparison with existing primitives, publication and sharing.

`import hw.sprite_chain` provides `@sprite_chain("actor")` with the existing
sprite destination/region/hex rows and `via "compiled"` or `via "push"`.
PUSH requires even byte width, opaque masks and explicit `irq "disabled"`.
The caller must keep maskable interrupts disabled throughout PUSH execution;
NMI/reentry is not supported. Both forms require owned writable code/data RAM.

Generated API: `actor()`, `actor_set_row_N(address)`, `actor_set_next_N(code_address)`,
`actor_set_entry(code_address)`, `actor_row_N_address()`, `actor_done_address()`.
Compiled row addresses point at the first byte; PUSH row addresses are end+1
(modulo 65536). Helpers accept raw addresses: runtime memory ownership, valid
acyclic continuations and atomic patch/draw scheduling belong to the caller.

Each row owns an address immediate and a mutable JP continuation. JP opcodes are
emitted as DB 195 / DW target because the current text peephole removes adjacent
JP instructions; direct emission protects their mutable locations. Native tests
exercise the instructions and patch words after optimization and assembly.

PUSH saves AF/BC/DE/HL on the ordinary stack, records that SP in a restore
immediate, then uses row destination SP and reversed LD DE / PUSH DE pairs.
The final section restores ordinary SP before popping registers and RET.
It does not change interrupt enable state. The code is nonreentrant.

First native checkpoint: 108 executions (two primitives × three modes × three
positions × six orders/subsets), plus two Nanz main setter/draw scenarios.
Independent full-RAM comparisons permit only the exact eight-byte save frame
and PUSH saved-SP immediate; exact return PC/SP and registers are checked.
See first-native.log. This initial positive matrix uses opaque four-byte rows;
mixed masks, rejection tests and full IRQ/ULA proof remain outstanding.
