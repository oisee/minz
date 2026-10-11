# Original CY errors: integrated semantic and native proof

Root fc7ca249 introduces explicit function/call CY metadata, typed call
status/code values, TermErrorRet, VM CallWithOutcome and native error code
preservation across result pickup/caller saves. Success clears CY after
return moves and frame cleanup; errors return code in A with CY set.

Original14 source unchanged: all bytepairs for safe_div/add/sub/compute/chain
and all byte indices for array_get.327936 calls per mode,983808 total
raw/final VM/native calls. Whole main checks stdout5250 for incoming CY0/1
and error1, full RAM/PC/SP. Root PASS51.442s. Full HIR/MIR2/Nanz
PASS23.642/21.783/12.060s.512 localframe/savedAF stress and malformed ABI/
outcome/trap/profile negatives also pass in focused core check.

Current ABI supports defined u8/void direct immediate checked calls.
Word/tuple/extern/nested forms and non-Z80 backends explicitly declined;
value-only profile schemas reject CY sources before execution. This is
a supported semantic domain, not arbitrary error handling or linked CP/M.

All133 preserved before/after/report payload SHA256 verified and copied to
share. Generated catalog now50 bounded independent VM/native paths,87
pending,194 explicit paths. Shared byte-word fusion remains in progress;
final goal remains active.
