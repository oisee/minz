# Next native milestone: bounded echo with explicit timeline extent

Selected after PR #268 merged as4db828c61 with9 green checks; downloaded CI arrangement artifacts matched fresh evidence. Work branch:spike/autosiril-native-echo. Native echo is not implemented yet; the following simulation is planning evidence only.

Read actual Ruby autooisee.rb apply_delays780+ and VM engine.nanz apply_echo.
A host planning simulation over a fixed65-slot array matched actual Ruby in10
cases, including the actual native Chronos ARRANGE stream. This does not prove
native execution. The source-bound planning receipt is saved alongside this plan; the temporary scripts remain at /tmp/minz-echo-planning until the reproducible native oracle is built.

Observed Chronos:2097 input LNotes →3215 delayed cells (1189/1013/1013),
all3 channel lengths1353; live pending maximum13. A dense wide-delay fixture
reaches all65 slots. Other cases cover original starts blocking earlier echoes,
release replacement/retention, older second echo beating newer first echo,
zero/equal/reversed delays, u/w flags, envelope renewal and large sparse gaps.

Ring rule: before consuming rowR flush pending rows<R; insert/replace the
original if destination is absent or original.type!='r', otherwise preserve
what earlier echoes put there. Schedule second then first from the ORIGINAL
input object, never from delayed output. Place only into nil/release cells.
A future original start restores its priority; an original release behaves like
the release/nil destination Ruby pre-cloned before scheduling. Therefore no
lookahead/full grid is needed. Effective delay<=64 (configured0..32, wide×2)
keeps all live rows within a64-row span; index=row%65 must have either no tag or
the identical absolute row. Never overwrite a different live row.

Centi-volumes1050/735 are valid for the producer's nominal1500 input; use the
actual typed Crystal echo_volume kernel. Ruby kind'e' renewal at these volumes
sets sample2/envelope15 under the current cool-envelope profile and preserves
note/ornament/type/kind/enote. Validate the producer's input invariants; do not
silently pretend the constant-volume kernel supports arbitrary input volumes.

Important extent boundary: ASIR kind3 carries nonempty LNotes/ornaments, not
trailing nil-array lengths. Ruby seq2vmod max_row uses ALL paired-note sources,
including unselected ones, and max(off,off+delay1,off+delay2). Do not infer length
only from sparse selected LNotes. A bounded target pass over the existing
NOTES.N/NOTES.BIN companion can compute max_off without storing notes (empty notes must retain Ruby base_rows1, not delay+1), then
open ARRANGE.A/ARRANGE.BIN. This retains all-source extent without host musical
logic or modifying the verified first3 stages. Chronos native NOTES max_off1340
plus max_delay12 gives base_rows1353, matching original Ruby exactly.

Proposed lossless output extension: ASIRv1 kind4, fixed16-byte tagged payloads;
pass kind3 LNote/definition roles0/1 and add role2 channel extent:u32LE metadata.
Emit all3 final channel extents, including empty/unvoiced trailing silence.
Validate framing/count/END/EOF and definition fragments. The full Chronos
candidate would be54880 logical bytes (3215 LNotes +9 definition fragments +3
extent records); this fits the existing TRD file limit. This is a sizing
calculation, not a native measurement. Row data remain u16 unless explicitly
versioned wider; check overflow, never wrap. CP/M can stream larger files than
TRD; preserve transport-specific capacity failures rather than shrinking music
inputs just to make both transports succeed.

Acceptance for the selected next goal:
actual Ruby/Crystal oracle, raw/final MIR2/native, sequential companion/input
files and CP/M/TRD bindings; exact collision priority, end extents and u/w/error
cases; fixed memory/physical guards/mutants, fresh/frozen CI and a separate PR.
Mix/pattern/VT2/VTX remain later stages. i7 read-only; no Herdr control outside
HERDR_ENV; preserve root user files and all integrated gates.


Implementation requirements to settle early:
- Strictly parse NOTES kind1 and ARRANGE kind3 on target, including definition
  fragment framing; no host note/pitch/extent computation. An unselected late
  source must still determine the initial global extent. With no paired notes,
  Ruby leaves max_row0 and base_rows1. With notes, use u32(max_off)+max(delay1,
  delay2)+1, independently of u/w; wide echoes may extend individual channels.
- New kind4 extent payloads use role2, channel byte1, row-count:u32LE at2..5,
  zero reserved6..15. The LNote row field remainsu16. Compute scheduled rows in
  u32 and explicitly reject data rows>65535 instead of wrapping; extent metadata
  can faithfully exceed65536 because unselected notes/trailing silence require
  no out-of-range LNote row. Validate record counts and terminal metadata.
- Nominal input volume1500, cool-envelope profile and configured delay0..32,
  wide/unvoiced flags are checked. Keep the producer's2046/512/1024 capacities.
  Echo can produce up to3 cells per input LNote; do not impose the old output
  event bound1024 on delayed cells. Keep CP/M and TRD file limits explicit.
- Pass every ornament definition unchanged; preserve all LNote fields and use
  the actual typed Crystal echo_volume kernel through the audited closed ABI.
- Full Chronos must match3215 delayed cells,3 extents1353 and all definitions
  against actual Ruby. Include collision priority, release/zero/equal/reversed
  delays,u/w/envelope, sparse gaps, full65-slot window, empty/unselected extent,
  malformed input and exact resource/overflow/I/O state cases.
- Deliver independently linked fourth images using real saved native companion
  and arrangement files, with fresh/frozen native/VM/CI proofs, memory/port/IRQ/
  mapping guards and restoring-write mutants. Measure code/data/spills/external
  RAM/stack/I/O/T-states. Preserve every integrated gate and user file, and merge
  only through a PR with required CI. API traps are not ROM/hardware/boot proof.
