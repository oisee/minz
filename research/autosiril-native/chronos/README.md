# Whole Chronos flatten experiment

Follow-up branch `spike/autosiril-native-chronos`; independent of the bounded
one/two-bank native-stage PR. It admits a three-bank note arena (2,046 records)
and1,024 output events per run, subject to the same checked image/stack window.

An initial probe over all1,485 actual Chronos paired notes matched all three
original mapped flatten channels:771/719/663 events,2,153 total. Every event,
error/lifecycle counter and memory guard matched raw/final MIR2 and assembled
Z80. Linked image25,340 B =6,962 code +18,220 declared globals +158 backend spill
bytes; observed below-entry stack74 B maximum.

This covers the entire paired-note input for this one song and all of its
flatten output. It is not the MIDI parser or the complete converter; later
LNote/echo/mix/pattern/VT2 stages remain outside this experiment.

```sh
python3 research/autosiril-native/verify_chronos.py \
  --autosiril /path/to/autosiril --out /tmp/minz-chronos-native-proof
```

The verifier rebuilds actual Crystal exports, runs the original full Ruby
converter oracle, uses its exact mapped channels as the reference, and separately
checks peak active notes from inclusive interval boundaries. Fresh proof and
extra capacity controls are required before this follow-up is merged.
