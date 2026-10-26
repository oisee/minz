# ADR 0053 Staged utilities and interchangeable stream transports

## Status

Accepted direction; implementation is staged. Date: 2026-10-26. Alice proposed
sequential utilities for Spectrum, CP/M and MOS. CP/M MIDI→note-file and
note-file→flatten-event-file are integrated through PRs #262/#263. The common
producer transport and first Scorpion TRD/API-trap stage are implemented in the
accompanying transport experiment. MOS file bindings, overlays and physical
TR-DOS ROM compatibility remain unimplemented/unproven.

## Context

Native MIDI parsing/pairing and paired-note flattening are now separately
checked programs. The first consumes256-byte input chunks; neither musical
algorithm needs its transport to be a Unix process pipe. Each phase may have
different workspace and code requirements. Requiring every later compiler
phase, all workspaces and disk services to reside together would unnecessarily
constrain an8-bit application.

The current paired-note program occupies29,990 bytes plus observed stack;
the complete Chronos flatten profile occupies25,340 bytes plus stack. A new
research composition fits the first two phases together at40,626 bytes with
the same2,046-note/1,024-output capacities, but that does not establish a
Spectrum disk-service memory layout or a complete MIDI→VT2 converter.

## Decision

Keep musical stages independent of transport. A stage consumes and produces
typed records through bounded reads/writes and explicit finish/error states.
The same algorithm may be used by a direct call/generator, an in-memory channel,
a sequential file utility or a loaded overlay.

The intended pipeline is MIDI bytes → paired notes → flattened events →
arrangement/echo/mix → tracker output. The final musical stages remain work to
do; the arrows do not imply that these native utilities already exist.

Start with sequential file-backed commands. Only one utility executes at a
time, so code/stack/private workspace can be reclaimed at process exit. This
does not require a multitasking scheduler or POSIX pipes. A future resident
dispatcher can load overlays while retaining selected arena regions in RAM,
provided its loader, layout and lifetime contract are verified separately.

### Stream contract

Reads/writes carry a byte count and explicit status: progress, EOF/finish or
error. An empty temporary chunk is not EOF. Borrowed input views expire before
the next refill, mapping change, overlay load or task switch. Persist handles
or copied records, never transport-buffer pointers (ADR0051).

File representations are versioned and target independent: explicit integer
widths/endianness, record kind/width, framing and a verifiable logical end with
counts. Native pointers, struct padding and generator frames are not wire data.
ASIR v1 retains the six-byte little-endian note format. Its 12-byte header is
`ASIR`, version1, kind1 (notes) or2 (events), payload width6 or5, flags0,
metadata:u16LE (source/channel count), reserved:u16LE0. A DATA record is tag01
followed by its fixed-width payload. END is tagFF, count:u32LE, ASCII `END!`.
Counts, widths, metadata, reserved bits and final physical padding are checked
by the CP/M consumer; the nonzero END signature distinguishes a missing end
record from zero fill. Golden vectors and malformed files are retained in
`research/autosiril-native/file-flatten/evidence/`.

The common first-stage binding packs count in the low16 bits and status in
the high16 bits of a u32: 0=progress, 1=EOF, 2=error. EOF must have count0;
unknown statuses are errors. `read_chunk()` fills a fixed buffer with at most
256 bytes, `write_chunk(count)` must acknowledge exactly the requested count,
and `finish(direction)` closes input0 or output1, returning zero on success.
`open_input()` supplies physical output chunk capacity and input-padding policy
with its own quantum (128 for CP/M,0 for exact-length TRD). Both file stages
track physical boundaries across short refills, rather than treating a chunk
boundary as a record boundary;
capacity is validated before any reading or output creation. Empty progress
retries are bounded (1024 stalls) rather than interpreted as EOF. These are
explicit compile/link-time bindings, not an implicit runtime scheduler.

Physical record/sector padding is outside the logical stream. In particular,
binary data must not use a text EOF character as its terminator. The source
SMF container has its own declared track lengths and terminal state; the disk
adapter must not feed physical padding as extra MIDI bytes. Short I/O, missing
end/count data and a genuine truncated stream must remain distinguishable.

### Platform bindings

CP/M is the first reference file transport to exercise with the current Z80
toolchain: separate transient programs and BDOS open/create/sequential read/write.
Each command must check I/O status and close/finish before the next stage opens
its output. No native Unix pipe facility is assumed.

MOS provides file handles and block read/write APIs. It can implement the same
record protocol, but MOS API availability and readiness of the complete MinZ
eZ80 application backend are separate claims. The current Nanz MOS library is
a console subset, so file wrappers still need implementation/verification.

For Spectrum, first exercise sequential stage files in a controlled TRD/API-trap
environment. Existing `zx.scorpion_disk` is a sector transport with explicit
paging assumptions, not an established high-level open-file API or physical
ROM proof. Code, stack and paging state needed across a disk call must remain
accessible while the mapped window changes. A generic all-RAM image linked at
0x4000 with stack near0xeffe is not automatically a valid disk-service layout.

Reserve ROM workspace and loader/runtime regions explicitly. If the note arena
is placed in an external bank/window, count its physical reservation separately
from loaded binary size. Verify restoration of mappings/interrupt policy and
short sector I/O before claiming real-ROM or hardware compatibility. An overlay
variant additionally needs a manifest that excludes persistent arenas and
dispatcher/stack regions from every load.

## Acceptance

1. Preserve the current native/VM/Ruby record oracles. Run first actual native
   MIDI→note-file and note-file→event-file utilities under CP/M transport; compare
   every record and the full Chronos three-channel result.
2. Exercise tiny chunks, record/sector boundaries, physical padding, empty input,
   invalid version/record width, malformed framing, truncation and short I/O.
3. Report code, resident/external data, buffers, stack and I/O separately for
   each stage. A smaller loaded file is not proof of a smaller total RAM budget.
4. Run equivalent stage files through the Spectrum sector/API-trap adapter with
   checked nonpaged code/stack and persistent arena placement. Keep physical ROM
   and hardware execution a separate acceptance step.
5. Keep the in-process route usable; transport selection should not fork the
   musical algorithms or require GC. Deliver implementations through PRs and
   ordinary required CI.

## Current evidence and remaining platform work

- `research/autosiril-native/files/`: first CP/M utility, 147 file executions
  plus standalone mze; original 342 streamed MIDI executions retained.
- `research/autosiril-native/file-flatten/`: second CP/M utility, 201 file
  executions and three native negative mutants; all 2153 Chronos events.
- `research/autosiril-native/transports/`: shared first-stage serialization,
  interchangeable CP/M/TRD bindings, explicit fixed/banked placement and
  adversarial transport/paging tests. This is API-trap evidence, not a ROM proof.
- `research/autosiril-native/stream-flatten/`: the second CP/M stage uses the
  same chunk API, CP/M binding and writer, including short progress chunks that
  split physical record padding across refills. Input padding quantum is
  explicit metadata, separate from output capacity.
- `research/autosiril-native/linked/`: the separate 40626-byte in-process path
  remains available. Its all-RAM layout is not the disk-service layout.

The original CP/M flatten utility remains as a separate reference; the common
API version now supplies the same sequential CP/M chain. Adding a TRD flatten
variant still needs a checked output allocation/length policy for a generator
whose final count is not known before exhaustion. The first TRD writer is
append-only; metadata publication is not crash-atomic. Its fixed workspace uses
screen RAM and its DI disk stub does not establish compatibility with a physical
ROM's workspace/interrupt ABI.
MOS remains an additional future transport, independent of full eZ80 readiness.

## Consequences

Sequential files trade disk passes and temporary storage for a smaller resident
program. A resident overlay route can reduce those passes but adds loader and
lifetime obligations. A direct in-process route avoids serialization when it
fits. All three can share record schemas, generators, scalar/meta kernels and
oracle fixtures; a scheduler is an optional later transport, not a prerequisite.

## References

- [ADR0051: region handles and banked arenas](0051-region-handles-and-banked-arenas.md)
- [ADR0052: demand-driven Autosiril](0052-autosiril-demand-driven-pipeline.md)
- [CP/M 2 system interface, original Digital Research manual](https://www.cpm.z80.de/manuals/archive/cpm22htm/ch5.htm)
- [Agon Platform MOS API](https://agonplatform.github.io/agon-docs/mos/API/)
- `stdlib/agon/mos.nanz`, `stdlib/zx/scorpion_disk.nanz`
