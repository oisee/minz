# Agon native frontend services

This extends explicit `-t agon -b z80 --runtime=agon-z80`. The ordinary shared
frontend catalog, picker, line/CSV conversion and table operations remain compiled
code. No CP/M CALL5 or host rendering/parser is emitted. The platform provider
uses typed finite intrinsics; no function name grants arbitrary ASM authority.

The six shared `_abap_fs_bdos_*` names retain their byte protocol for compiled
callers. Agon translates actual MOS file/directory results to that protocol;
`bdos` in these common symbol names does not select CP/M on Agon.

## Files and directories

One private FIL object is opened read-only. No MOS global filehandle pool or
caller filehandle is borrowed. Each read requests exactly128 bytes with a full
24-bit count, checks FRESULT and the returned24-bit count, and pads the partial
last record with ASCII26 after reading. Return0 supplies a record,1 is EOF,2 is
an I/O error. Open/close use0success and255failure. Shared compiled code owns
CRLF/LF/ASCII26 handling, field conversion, capacity checks and table storage.

Public frontend names are one ASCII8.3 basename, optionally the shared A: alias.
A: maps to the current MOS directory on the selected SD volume; subdirectories,
absolute paths and other drive letters are refused. The low-level native open
also bounds its13-byte name buffer and validates a single basename without
wildcards or path separators. It never sends an unterminated pointer to MOS.
Directory calls use the fixed current-directory path `.`.

The target provider compiles primary-name8.3 validation, case normalization,
FCB wildcard matching, deduplication and deterministic ordering. It excludes
volume labels/directories and long primary filenames instead of exposing an
ambiguous alias. It stores at most128 canonical names. A129th match closes the
iterator and raises table-capacity fault7. Separately, at most4096 raw directory
entries may be examined, plus the EOF probe. A larger directory, provider read
error or close error returns254 to the common catalog's existing error branch;
it clears the private catalog and never exposes a successful partial catalog.
EOF is255. Ordinary entries use DMA slot0, with an encoded11-byte FCB name.

The ABI requires owned mutable arrays: DMA128, filename13 and FILINFO278. The
compiler rejects missing, undersized, fixed-address, constant and wrong-element
buffer layouts. All firmware pointers use the physical application bank040000;
all firmware counts are initialized with explicit24-bit instructions. No binary
pointer patching or raw-byte instruction workaround is used.

## Firmware effects and cleanup

File/directory leaves preserve BC/DE/HL16 and full IX/IY24 around genuine
RST.LIS08 calls. Result A is a byte; flags are volatile. The short stack is
resident; MOS calls use the separate long stack. Terminal cleanup closes owned
file/directory objects before restoring the original full IX/IY and mixed caller
frames. Status6 denotes a native resource-close failure; status4 also covers the
private file/directory guard checks. Other statuses follow the core ABI.

FIL36, DIR49 and FILINFO278 follow Console8v2.3.3 FatFS configuration
(FF_USE_FIND1, FF_USE_LFN2, FF_MAX_LFN255, FF_FS_TINY1, FF_FS_EXFAT0,
FF_FS_LOCK0). SDK mos_api.inc omits DIR's3-byte find-pattern pointer. The official
release source tag454e32a5c75cc0373de27ffd09667dc13f36f651 is supporting source
information; its release binary differs from the bundled pinned MOS image and
is not claimed as a reproducible source build of that image. Actual pinned
binary/map, native API observations and object canaries remain evidence.

The console-only512-byte long-stack allowance is insufficient for stack-local
FatFS LFN storage. Frontend execution requires SPL in0BFE00..0BFFFF, reserving
at least3584 bytes above0BF000, including the3072-byte filesystem allowance and
512-byte firmware/interrupt allowance. This is a stronger pinned-environment
precondition, not certification of arbitrary firmware handlers. Native canary
sampling measures tested paths; it does not establish an arbitrary maximum.

## Keyboard

The nonblocking native leaf obtains the actual MOS system-variable pointer via
RST.LIS08/function08. It snapshots event counter, key-down, virtual key and ASCII
with a counter retry, ignores key-up/stale events and preserves full IX/IY. Actual
VDP F4/Up/Down/Enter/Escape events translate to the common compiled key protocol;
the short ESC sequence is queued inside the adapter. It does not inject semantic
keypress bytes from the host. The pinned legacy firmware has one latest-packet
state, so bursts exceeding that firmware's event delivery/consumer rate are not
claimed lossless. Newer MOS keyboard queue API62 is not assumed available.

## Evidence boundary

`--sdcard` host directories intercept FatFS services in the emulator and are
useful preliminary API tests only. Filesystem/native-stack evidence uses
`--sdcard-img` raw FAT32 media, which disables hostfs and exercises the real pinned
MOS FatFS/SPI path, full eZ80 execution and full VDP with real SDL events.
Physical SD hardware is a separate claim. The native final SALV grid and remaining
negative/cancel/error gates must pass before the full F4 CSV ALV goal is complete.

SALV uses the shared compiled 80-column, 24-row layout and key decoder. Its Agon
adapter emits BBC VDU commands: reset text viewport (26), clear (12), set text
viewport (28, left0/bottom23/right79/top0), hide cursor (23,1,0), and position
cursor (31,x,y). Normal completion resets the full viewport, shows the cursor,
and places subsequent MOS output below the grid. It does not send ANSI escapes
to the VDP. The pinned default VDP mode provides at least80columns and24rows;
changing video mode while a report is running is outside this adapter contract.

The screen lifecycle uses finite no-argument, byte-result compiler intrinsics
`@mir.agon.screen.begin` and `@mir.agon.screen.end`; ordinary compiled cursor
placement uses the existing canonical byte console leaf. The owned screen-active
byte makes cleanup conditional. Fault cleanup clears that byte before emitting
the reset sequence, so a console failure during restoration cannot recurse
indefinitely. Native filesystem and screen cleanup run on the reset reserved
application short stack, with the saved long-stack frame restored; the caller's
short stack is restored only after those calls finish. This preserves caller
words even at the admitted lower short-stack boundary beside a full-size image.

SALV navigation maps the pinned legacy virtual keys to the shared byte protocol:
F3 produces ESC O R, Home/End produce ESC [ H/F, and PageUp/PageDown produce
ESC [ 5 ~/6 ~. Main and keypad variants share each mapping. Up/Down and F4 keep
the existing sequences. The compiler-owned pending queue holds three tail bytes
for a four-byte sequence and is drained before accepting a later MOS packet.
The underlying MOS sysvars expose the latest packet rather than a lossless key
queue; this adapter does not guarantee recovery of packets overwritten before a
poll. Native directed tests use genuine SDL events with observed renderer frames
between key presses.
