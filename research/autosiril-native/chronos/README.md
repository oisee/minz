# Whole Chronos flatten experiment

Follow-up branch `spike/autosiril-native-chronos`; independent of the bounded
one/two-bank native-stage PR. It admits a three-bank note arena (2,046 records)
and1,024 output events per run, subject to the same checked image/stack window.

The fresh gate over all1,485 actual Chronos paired notes matches all three
original mapped flatten channels:771/719/663 events,2,153 total. Every event,
error/lifecycle counter and memory guard matched raw/final MIR2 and assembled
Z80. Linked image25,340 B =6,962 code +18,220 declared globals +158 backend spill
bytes; observed below-entry stack74 B maximum over the actual song (80 B
including the added page-boundary controls).

The three song channels plus four capacity controls execute on all three
backends: **21 target executions**. The controls cover the first and last record
of the third page, rejection of declared handle2,047 before payload reads, and
exactly1,024 output events. Four transport negatives reject excess notes, demand,
banks and output storage. Exact-limit consumption stops at yield before the
last removal suffix; one active handle remains, with the cursor lease released.

This covers the entire paired-note input for this one song and all of its
flatten output. It is not the MIDI parser or the complete converter; later
LNote/echo/mix/pattern/VT2 stages remain outside this experiment.

```sh
python3 research/autosiril-native/verify_chronos.py \
  --autosiril /path/to/autosiril --out /tmp/minz-chronos-native-proof
```

The verifier rebuilds actual Crystal exports, runs the original full Ruby
converter oracle, uses its exact mapped channels as the reference, and separately
checks peak active notes from inclusive interval boundaries. Capacity music
references use the original Ruby interval adapter. The source/input/export
hashes, full outputs and assembly are retained in `evidence/`. CI replays the
seven frozen cases without requiring Ruby/Crystal tools:

```sh
go -C minzc test ./cmd/autosiril-native -count=1
```

Native channel costs are180,203,227 /162,403,129 /164,320,283 T-states; this
demonstrates correctness and fit, not realtime conversion. The gate uses a flat
resident arena with simulated mapping, not physical bank switching. The source
algorithm and its512-active-handle limit remain unchanged.
