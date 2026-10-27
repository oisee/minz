# ASIRv1 kind3: arrangement records

Header12 bytes: `ASIR`, version1, kind3, payload width16, flags0,
channel-count:u16LE3, reserved:u16LE0. Each DATA record is tag01 +16 payload
bytes; END is tagFF + DATA-record-count:u32LE + `END!`. Exact logical EOF on
TRD; CP/M permits only zero fill to the final128-byte record boundary.
All integer fields are little endian. No field is a native pointer.

Two payload roles share the fixed width:

| Offset | role0: LNote | role1: ornament fragment |
|---|---|---|
|0|role0|role1|
|1|channel:u8|definition id:u16 (bytes1..2)|
|2..3|row:u16|total definition length:u16 (3..4)|
|4|pitch:u8| |
|5|sample:u8|fragment offset:u16 (5..6)|
|6|envelope:u8| |
|7..8|ornament:u16|count:u8 at7; data begins8|
|9..10|centi-volume:u16|data continued|
|11|ASCII type s/r|data continued|
|12|ASCII kind m/p/d/e|data continued|
|13..14|envelope pitch:u16; FFFF means absent|data continued|
|15|reserved0|last of8 fragment bytes|

Ornament fragments have count1..8, offsets0,8,... and zero unused data bytes.
Definitions are interned by exact expanded bytes in Ruby discovery order,
starting with repeated-zero definition0. A new definition is emitted completely
before the cell which discovers it. Definitions are emitted even for mono/drum
cells, matching Ruby's ornament interning before per-channel choice. A mono
channel's configured ornament is a literal preset selection; poly references
use the interned definition id.

LNotes are ordered by channel then row, one per input row-group, retaining all
original poly pitches for clipping/ornament construction. END counts BOTH
payload roles, not just LNotes. Separate counters report incoming events,
LNotes, definitions and pooled bytes. Publication follows the existing transport
finish policy; this adds no crash-atomic catalog/system transaction.
