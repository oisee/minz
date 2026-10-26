# Sequential native CP/M file chain

The two independent transient programs now exchange actual disk files:

```text
INPUT.MID --PAIR.COM--> NOTES.BIN --FLATTEN.COM--> EVENTS.BIN
```

Each stage opens/reads/closes its input, then creates/writes/closes its output
through BDOS CALL5. Separate emulator processes run the standalone commands;
only files persist between them. The target reads the note envelope and records,
checks framing/resource bounds, then invokes the unchanged shared lazy flatten
generator with the actual Crystal `cell_action` and `prefer` scalar functions.
The host implements OS record I/O and inspects results; it supplies no musical
calculation, preloaded note arena or precomputed generator output to either
standalone command.

The current arrangement profile matches the Chronos oracle: output channel0
uses source1/mono-max, channel1 source0/poly, channel2 source0/mono-max. The
producer `.COM` bootstraps eight rows per beat. These are explicit prototype
settings, not a complete Autosiril command-line/configuration implementation.

## Evidence and reproduction

```sh
PYTHONDONTWRITEBYTECODE=1 python3 research/autosiril-native/file-flatten/verify.py \
  --autosiril /path/to/autosiril --out /tmp/autosiril-file-chain
```

For macOS's unrelated current-main `Select` portability issue, pass the optional
`--go-overlay` described in `../files/README.md`. The fresh parent proof records
the overlay and replacement source hashes; no local user file is committed.

The fresh gate runs the prior342 MIDI executions,147 file producer executions
and **201 flatten executions** across raw MIR2/final MIR2/native Z80. The latter
are67 cases:21 actual files emitted by the native producer, five interval/resource
fixtures checked against original Ruby, and41 malformed/OS cases. All1,485
Chronos notes and2,153 events (771/719/663) match Ruby; the original full MIDI Ruby
path and independent quantized-interval Ruby adapter also agree. Standalone
`PAIR.COM` then `FLATTEN.COM` in ordinary `mze -t cpm` produce identical files.

`replay.py` reconstructs exactly that fresh-backed matrix in required CI, using
the completed producer replay's actual files and frozen original Ruby results.
It checks the exact matrix and composed source/driver hashes before execution,
then runs201 target executions, three mutation rejections and the standalone
chain. No Ruby/Crystal installation is needed for this frozen CI replay. Full
generated sources, assemblies, images, MIR, per-case outputs and logs are retained
by CI; checked-in receipts identify the fresh proof and its source/oracle hashes.

The reader rejects invalid magic/version/kind/width/flags/reserved fields, source
counts above64, invalid note pitch/source, unknown tags, wrong END count/signature,
missing END/count, truncation, nonzero/excess physical padding and note2047.
Existing semantic validation rejects off-before-start and unsorted source rows.
Tests cover record boundaries, all three arena pages,2,046 notes,512 active notes,
513 active-note rejection, exactly1,024 output events in a channel, and overflow.
The consumer pulls once beyond its output capacity to distinguish a complete
1,024-event channel from truncation, and completes generator cleanup on exact
capacity. OS failure cases include the extra EOF probe and both close operations.

Native guards check code/data/stack spans, PC, return ABI, canaries, immutable
arena padding and absence of unexpected port I/O. The full note arena is observed
after load/validation and must remain read-only during flattening; native write
counts catch transient writes even if restored before return. `negative.py`
verifies rejection of such a restoring-write mutant, a capacity-guard bypass and
a removed END-signature check.

## Format and budgets

Both files use the common12-byte ASIR v1 envelope described in `../files/README.md`
and the explicit `ff count:u32 END!` logical terminator. The nonzero terminator
and final signature distinguish missing completion data from physical zero fill,
including an empty file. No native pointer, struct padding or generator frame is
serialized. This is still a bounded record protocol, not a general filesystem API.

For EVENTS.BIN the header kind is2, payload width5 and metadata at8–9 is the
number of output channels (3). A DATA record is:

```text
01 channel:u8 row:u16 value:u16
```

`value` packs pitch in its low byte and action in its high byte (start1/release4).
Records retain channel order and Ruby's row/event order. END counts all DATA
records. Only the remainder of the last physical128-byte record is zero padding;
an extra whole record is rejected. The Chronos note file occupies10,496 physical
bytes (10,416 logical); its event file occupies13,056 (12,939 logical).

| Program | Loaded image | Code/bootstrap | Globals | Static spills | Max kernel stack |
|---|---:|---:|---:|---:|---:|
| PAIR.COM | 30,914B | 15,411B | 15,063B | 440B | 66B |
| FLATTEN.COM | 22,750B | 8,205B | 14,380B | 165B | 63B |

Stack figures exclude the initial two-byte return address. Both images include
the full12,288-byte note arena reservation, including bank padding; no external
arena memory is hidden from these budgets. Output events stream through128-byte
DMA rather than an output array, retaining the1,024-event per-channel bound.
Chronos's standalone commands take450,012,959 and446,711,944 CPU T-states, excluding
host disk-service time. These are offline proof measurements, not real-time
performance or a complete tracker converter benchmark.

The separate40,626B in-process route remains available under `../linked`.
Spectrum TRD/paging, physical ROM/hardware compatibility, MOS file wrappers,
arrangement/echo/mix and final native VT2/VTX generation remain work to do. The
current `.COM` research bootstraps halt with the target result in HL; they do not
yet provide a portable shell exit convention. I/O failure can leave partial files
and is always a nonzero target status; atomic output publication is not provided.
