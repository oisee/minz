This is an isolated WIP for the closed native call-spill snapshot certificate.
No production merge or native execution has occurred in this certificate lane. Source files add a
distinct typed save/restore stage grammar and exact E3 stack exchange recipes.
The previous A-byte result-pickup grammar and generic stack permissions remain
unchanged. E3 is not classified as a generic frame opcode.

Resolver binding includes original MIR function/block/call identity, live-after
home excluding result definitions, native width, generated symbol and actual
allocation. The first certificate supports actual allocated LocMem homes;
dynamic generated overrides require further independently bound sidecar checks.
Save/restore stages are paired by callsite/home; runtime owns LIFO activation
slots, checks SP, call depth, initialized HL role, saved bytes and return-frame
exclusion before any exchange. Actual byte order is low read, high read, high
write, low write. Pre-HL writes and post-HL values are checked.

The actual intervening direct CALL has its own marker, whose bytes and target
are checked against the assembled binary and original MIR symbol. Restore
requires that this call executed in the same activation and returned before
the saved slot can be consumed. Exchange checks also preserve F explicitly.
Annotation composition uses exact comment tokens; it retains existing memory
sidecar checks when a load/store carries both annotations.

Focused binder twelve forgeries and existing native-memory/sidecar checks:
PASS pipeline 0.016 s, MIR2 0.015 s. They reject wrong call identity/index,
result/nonlive homes, wide integer truncation, missing markers, wrong opcode,
unpaired stages, wrong allocation, mismatched CALL binary/target and marker
prefix borrowing. Native execution has not occurred here.

Collector reset/terminal/overlap/hash plumbing and lifecycle-before-effect
negative tests are owned separately by the pipeline agent. Remaining before
release: one bounded protected native collect/replay gate in the combined
branch, final saved evidence review and production gates. The private
certificate currently accepts normal direct CALLs and allocated one/two-byte
LocMem homes; unsupported dynamic overrides and indirect/fixed-address scopes
refuse without granting stack permission.
