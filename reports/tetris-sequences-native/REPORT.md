# Original Tetris spawn, hold, and persistent composed sequences

The previous twelve-entry packet remains unchanged. This follow-up pins the same complete original CP/M/TUI sources and dependency hashes, then adds actual `spawn_piece`, `hold_piece`, and `do_lock_and_clear` calls. Source/data/domain freeze4ee99f30 precedes execution. Model dependency hashes identify the earlier independent geometry/arithmetic model; new expected transitions come from a separately implemented state machine, not compiled output.

Book v7 §2.4 globals, §2.6 arrays/pointers, §3.1 numeric widths define stored state. The original source defines the concrete game policy. The independent model intentionally retains its details: spawning advances the LFSR/next queue, resets rotation/position/hold flag, sets but does not clear game_over on collision; empty hold invokes spawn (therefore resets hold_used again), nonempty hold exchanges types without collision or RNG, and only hold_used==1 refuses. This is a proof of the actual algorithm, without substituting another Tetris rule set.

Frozen broad domains per source/mode:

- spawn: all seven next types × eight LFSR seeds × three boards × both sticky game_over states, 336 cases.
- hold: all seven current types × empty/all seven held types × hold_used0/1/2 × eight seeds × two boards, 2,688 cases.
- composed lock/clear/score/spawn: seven types × four rotations × three boards × eight seeds, 672 cases. Original complete call closure executes; no externally precomputed return bypasses it.
- 112 persistent eight-call sequences: spawn, hold, hold, rotate, ghost query, composed lock/clear/spawn, hold, composed lock/clear/spawn. Each checkpoint verifies independently modeled full state and ordered board effects. The test controller explicitly applies the independently modeled ghost descent to cur_y between the query and locking; this is a declared workload state input, not an original source API or device simulation. VM/native RAM otherwise persists across calls and is not reinitialized between sequence steps.

Total 4,592 actual entry calls per source/mode (counts include 896 sequence entries). All six combinations PASS 46.146 s: **27,552 assembled native calls and 55,104 raw/final VM calls**, plus 672 persistent sequences. The entire original module/import is parsed and emitted. Native fetch is restricted to the actual computational closure; ASM/indirect/device calls are refused. Actual allocated entry ABI, complete 64 KiB state, full code image, exact board reads/writes, return PC/SP, and exact private stack/spill bounds are checked. Expected state includes next/held types, RNG state, score/line count/speed and game-over flag; no compiler-derived expected values.

Independent controls separately check empty hold's reset, used-hold refusal, nonempty hold's absence of RNG/collision and blocked spawn. No new production bug emerged. Full interactive main, rendering, keyboard/BDOS/TUI runtime remain outside execution scope. This follow-up does not claim native device execution or change source semantics.
