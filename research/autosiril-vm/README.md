# Autosiril: mixed Crystal + Nanz, VM first

Branch `spike/autosiril-vm`, based on merged PR #252. The target for the first
functional port is MIR2/MZV with generous memory. Z80 RAM, disk transport and
banking are subsequent milestones; this is not yet a complete VT2/VTX converter.

## Current verified milestone

`engine.nanz` reuses the real Nanz SMF decoder, pairs notes FIFO by track/channel/
pitch, normalizes velocity-zero releases, closes pending notes as midilib does,
constructs the timeline grid and implements Autosiril's poly/mono/drum stage.
`music.cr` contains musical kernels compiled by the **native Crystal semantic
frontend**, exported to Lanz and linked as ordinary HIR functions. Nanz calls
these functions for tick-to-row quantization, cell action and pitch selection.
They are not host callbacks. The loader refuses unresolved functions, fallible
bindings, symbol collisions and signature mismatches.

Host Go reads files, parses command-line mapping syntax, feeds bytes and
serializes snapshots. Pairing, timeline construction, quantization and musical
selection execute in MIR2. Ruby is only an independent reference: the oracle
loads the original `autooisee.rb` classes/functions before its main section and
reads its MIDI assets without writing into that checkout.

The native Crystal scalar exporter gained UInt32 values/literals and unsigned
integer division/remainder. Signed division remains rejected because Crystal
floor division and truncating integer division differ for negative values.

## Evidence

Six standard mappings (chronos, flim, three imrav variants, tottoro) and five
synthetic fixtures run in raw and optimized MZV. All paired-note fields and the
exact row/pitch/type/order of nonempty flattened cells match Ruby. Across both
engines this includes **39,426 flattened cells**. Synthetic controls cover
same-pitch overlap/FIFO, velocity zero, pending notes at EOT, zero-length notes,
channel separation and cells mixing continuation/release/retrigger.

The VM heap is about **31.4 MB** and real accesses exceed **30 MB**, exercising
existing MZV wide-global addressing. Explicit language pointers keep their
16-bit ABI: small decoder descriptors/backing bytes are placed first and their
addresses checked. Large tables use ordinary indexed globals with u32 indices.
No new host-backed musical containers or widened fake pointer ABI are used.

The large tables are deliberately generous, fixed-capacity prototypes, with
explicit exhaustion errors. Quantization uses unsigned 32-bit values and valid
SMF PPQN/row settings; this is not a claim of arbitrary-precision Ruby integers.

Reproduce from repository root (Ruby + midilib, Crystal with its compiler
library/LLVM, and Go):

```sh
python3 research/autosiril-vm/verify.py --autosiril /path/to/autosiril
```

Outputs default to `/tmp/minz-autosiril-vm-results`. MIDI assets are not vendored.
Receipts include hashes of actual sources and inputs. The previous native
Crystal scalar/outcome corpus was rerun successfully, and Go tests cover mixed
scalar ABI rejection and result storage above 128 KiB.

## Next stages

LNotes and sample/envelope mapping, ornaments, echo, downmix and VT2 rendering
remain to be ported. The source Autosiril's native output is VortexTracker II
text (`.mide.txt`), distinct from a VTX AY-register stream. No complete module
conversion or audio playback is claimed at this milestone. A pleasant Crystal
collection facade and Z80-sized storage can follow functional equivalence.
