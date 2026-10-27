# The 62-row native_chronos vs original-render divergence, pinned (checkpoint 3)

Checkpoint 2 reported that the planning `native_chronos` rows (native
chain) differ from the original full Ruby pipeline render of
`test/chronos.mid` in 62 of 1352 rows, as an unexplained upstream question.
This checkpoint pins the exact first difference and its mechanism.
Reproducer: `chronos_divergence_repro.rb` in this directory (read-only
against the original tree, original source guarded by exact SHA256);
evidence: `evidence/checkpoint3-chronos-divergence.json`.

## Verdict in one paragraph

The invocation settings are IDENTICAL (both renders derive
max_row 1352, pattern_size 64, skip_lines 64, per_beat 8, per_delay 6,
per_delay2 12, orn_repeat 2, max_offset 6, detect_key 0, ppqn 120,
cpr 15.0, chan_settings [me,p,m], sources [1,0,0]). The native render
stage is faithful: it reproduced its kind5 input exactly (its own stage
proof is not contradicted). The divergence is INSIDE the native chain's
kind5 fixture itself: the native mix-stage output
(`cases.json` entry `native_chronos`, 54912 B, SHA256
`cceabdce…a12d8d3cf`, consumed as `native_inputs[0]` by the render stage)
differs from the actual Ruby `downmix` result in exactly 62 cells — and
those 62 cells are exactly the 62 differing rendered rows. This is a real
content divergence of the native chain (first candidate: the `p`-channel
path around the pmod→lmod collapse), NOT a fixture-profile or invocation
difference.

## First difference

Row 259, rendered line block `noteB` (channel index 1, the `p` channel;
columns 22 and 28 of the 49-column row):

```
original: ....|..|G#2 2F.7 ....|A#3 2F.F ....|A#3 2F.F ....
planning: ....|..|G#2 2F.7 ....|D#3 2F4F ....|A#3 2F.F ....
                                     ^^ ^      note A#3→D#3, ornament .→4
```

All 62 differing rows are 259, 262, 265, 271, 274, 277, 395, 398, 401,
407, 410, 420, 424, 426, 428, 429, 432, 433, 434, 435, … (last 1337), all
in `noteB` only; `env`, `noteA` and `noteC` are byte-identical everywhere.

## Mechanism

Cell-level diff of the kind5 fixture against the actual Ruby `mmod`
(volumes scale ×100 in the wire): exactly 62 cells differ, ALL on channel
1, all `{note 51, ornament 4, envelope 0, kind 'p'}` in kind5 versus
`{note 58, ornament 0, envelope 0, kind 'p'}` in Ruby. No missing or extra
cells; channels 0 and 2 are cell-identical.

What Ruby does at channel 1, row 259: the pmod cell holds the dyad
`{FNote 58, FNote 51}` (both source track 0, which channels 1 and 2 both
render — `sources = [1,0,0]`). `pmod2lmod` computes proto-ornament offsets
`[0, 7]`; `squize_ornament` takes the mediana 7, and the `max_offset 6`
filter DELETES offset 0 (|0−7| = 7 > 6), leaving `[7]`; rebasing by the
minimum gives base note 51+7 = **58** with normalized ornament `[0]` =
`"L0,0"` = ornament id **0**. That is the Ruby cell (58, orn 0) → `A#3`
+ `.`. Channel 2 (`m` channel, same dyad) shows the same collapse to 58 —
and the native chain's channel 2 cell is CORRECT (58), matching Ruby.

What the native chain's kind5 cell implies: ornament id 4 is, in the REAL
Chronos ornament map, `"L0,0,1,1"` — offsets `[0, 1]`. So the native
`p`-channel path collapsed a dyad `{51, 52}` (a semitone), not Ruby's
`{51, 58}` (a fifth): the second chord member itself was 52 where the
track/render has 58, i.e. the divergence enters the native chain at or
before the pmod→lmod collapse for the `p` channel — after the shared
flatten (channel 2 renders the same source row correctly) and before the
mix-stage output. The wrong note then propagates into the echo-decayed
copies (volumes 1050/735 = 10.5/7.35) at rows 271–277, 407–435 etc.,
which explains the full 62-row tail.

Note the native render stage was proved equal to Ruby render **on the same
kind5 input**, so these proofs remain valid; what was never proved is
native-chain kind5 == actual Ruby `mmod` for Chronos end-to-end (the echo
stage's retained-linked audit compared only `per_beat/sources/notes/
max_row/flattened`, not full mixed cells).

## Reproduce

```sh
ruby research/autosiril-native/vt2/chronos_divergence_repro.rb \
  --autosiril /Users/alice/dev/autosiril \
  --planning /tmp/minz-render-planning-v4
```

Output JSON fields: `settings_derived_by_original_setup` (identical for
both renders), `first_differing_row` (row 259, block noteB, cols 22/28),
`differing_rows` (62), `kind5_vs_ruby_mmod` (62 cells, all channel 1,
first cells shown), `squize_analysis_first_cell` (Ruby replay: dyad
{58,51} → offsets [0,7] → mediana 7 filter keeps [7] → base 58, ornament
"L0,0"; versus the kind5 cell 51/ornament 4 = "L0,0,1,1").

## What should NOT happen

The original Ruby, prior goldens and target code are unchanged; nothing
here alters or hides the mismatch. The fix belongs to the parent's
upstream coordination (the native `p`-channel stage), not to this VT2
step.
