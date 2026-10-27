# Kind5 wire contract (CP/M producer tested; TRD pending)

Kind5 adds original render timing to the lossless kind4 vocabulary. This document
is implemented by the current CP/M stage; the TRD producer remains pending.
All multibyte integers are little endian. A logical stream has:

1.12-byte header `ASIR 01 05 10 00 03 00 00 00` (hex bytes).
2.Zero or more DATA records: tag01 followed by16 payload bytes.
3.END: tagFF, u32 DATA-record count, ASCII `END!`.

Only physical record/sector padding may follow END; it must be zero. END's
record count includes every definition fragment, cell and metadata record.

Payload roles and canonical order:

| Role | Meaning | Payload |
|---|---|---|
|1| Ornament fragment | Exactly kind4's unchanged16-byte definition payload |
|0| LNote | Exactly kind4's16-byte cell payload; byte1 is destination0..2 |
|2| Trimmed channel extent | byte1=channel0..2, bytes2..5=u32 extent, bytes6..15=0 |
|3| Original render rows | byte1=0, bytes2..5=u32 original max_row, bytes6..15=0 |

All definition fragments precede cells. Cells are channel-major, with strictly
increasing rows within a channel. After the last cell, exactly three role2
records occur in channel order0,1,2, then exactly one role3 record, then END.
No cells/definitions can follow metadata. A channel with no cells has extent0;
every other extent is its last cell row+1, computed inu32. No other roles exist.

Role3 is the original global `$set.max_row` from NOTES and delay configuration,
not the echo input's array extents, nor the maximum of mixed extents. Ruby's
render loop uses `max_row.times`: do not add one here. Empty NOTES has max_row0.
A wide echo can enlarge kind4 extents beyond this value; a mixed spill can also
outlive the render range. Preserve these differences rather than normalizing.

The selected input profile has3 source lanes, at most3072 cells per lane and9216
total. Mixed output may put all9216 cells in one destination; there is no3072
per-destination or old1024 cap. Definitions remain at most64 and4096 expanded
bytes. Row addition usesu32 and faults before emitting/casting DATA row>65535.
Role2/role3 metadata is unsigned32-bit, never silently truncated tou16.

All actual LNote fields survive clone/spill unchanged except destination channel
and row. Definitions are copied without renumbering. Equal volume keeps the
earlier source, and spill candidates use nil/release as specified by original
Ruby. Fixed source-order grouping is configured separately from the wire.

Publication follows each transport's explicit append/make/close contract. A
failed producer's visible exact prefix is not a valid kind5 file. CP/M's128-byte
padding and large extents are independent of TRD's255-sector/space limits.
