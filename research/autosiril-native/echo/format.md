# ASIRv1 kind4: delayed sparse channels with final extents

Header: `ASIR`, version1, kind4, payload width16, flags0,
channel-count:u16LE3, reserved:u16LE0. DATA is byte01 plus16 bytes;
END is byteFF, DATA-count:u32LE, `END!`. TRD requires logical EOF;
CP/M permits zero fill only to the final128-byte record boundary.
No field contains a native pointer.

Roles0 and1 retain the [kind3 layout](../arrange/format.md) exactly.
Role0 cells are ordered by channel/row and contain original or delayed LNotes.
Each ornament definition is copied without changes. The definition stream can
appear between delayed cells: fragments retain their input order and are emitted
when read, before processing the next original cell. They can therefore precede
pending echoes of the previous channel. Definition availability still precedes
all references to it. END counts every role, including extent records.

After every cell/definition comes exactly one final extent for each channel,
in order0,1,2:

| Byte | Meaning |
|---|---|
|0|role2|
|1|channel:u8|
|2..5|channel row count:u32LE|
|6..15|reserved zero|

The companion NOTES stream is read and closed first. Empty notes give base
extent1. Otherwise base extent is `u32(max_off)+max(delay1,delay2)+1`, including
all sources even when unselected or unvoiced. Each accepted original/echo can
extend its channel to row+1. Extents can exceed65536 for trailing silence;
DATA rows remainu16, and scheduled rows>65535 are rejected before casting.

Input profile: nominal centi-volume1500, configured delays0..32, wide doubles
the delay, unvoiced suppresses scheduling. The actual Crystal export computes
first/second centi-volumes1050/735. Other input volumes are rejected explicitly.
The cool-envelope profile renews delayed kind'e' cells to sample2/envelope15;
original e cells must match the verified envelope tables. Source kind must match
the configured m/p/d mode, or e. Poly ornament ids refer to completed definitions.

Limits:2046 paired notes;64 source ids;1024 input LNotes/channel;
3072 delayed cells/channel;64 definitions/4096 expanded definition bytes.
Pending cells occupy65 tagged slots, spanning at most64 rows. No full song
array is allocated. TRD output length/sector/space limits remain independent
of these musical limits; CP/M may produce files larger than64KiB.

Publication retains the transport's existing semantics, including possible
partial output on failure. API-trap experiments do not establish ROM/FDC,
physical hardware, boot, CP/M TPA/CCP, MOS/eZ80, or complete VT2/VTX support.
