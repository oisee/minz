# Original hashmap baseline and exclusive PC attribution

`baseline/` preserves current parent `1d4d3d10bdc0ea51609a3d695bd626f2294e3d07`
36-call evidence; the focused test passed in 0.304s. `history/ef270-baseline/`
retains earlier unchanged-source evidence produced by `ef270333`. Both give: 138440 T / 1013 B, SHA256
`aefde17ab12e824e4d790cc7147f0974329faf5dded51f032643eb316d56ba86`.
JSON/MIR/HIR dumps retain the complete oracle, raw/pre/post and actual target
state/access observations. ASM and test patch use exact-byte gzip with zero mtime.
Historical nested manifests describe their original packages; newly compressed
ASM/patch paths are bound by the later root stage manifest.

`verify_baseline.py` is the independently reviewed sequential Python oracle,
adapted only to pack-relative paths. Run `python3 verify_baseline.py`; the retained
`independent-verification.txt` is terminal PASS. It checks exact original source,
36 persistent results, complete 82-byte global state and every global byte's
read/write count on raw/pre/post and actual target, plus binary SHA/T/bytes.
The original independent review documents concrete coverage limits and sentinel
`map_has` behavior; no ordered native event or trap proof is claimed.

`pc/` retains the read-only instrumentation overlay, replay/attribution helpers,
function/block costs and PC records. Attribution excludes bootstrap, agrees with
exact uninstrumented binary/state/timing, and counts physical function ranges
exclusively rather than charging callees twice. This pack has not changed the
compiler or program. No optimized winner or gain is claimed.

Current-parent focused baseline gate passed; source, ASM and binary are exactly
equal to history. No new broad suite was required for the added test-only change;
the prior production pipeline gate remains separately recorded by root. Combined
stage manifest and publication are pending root work.

For PC reproduction against the original ef270 reference, extract
`history/ef270-baseline/baseline.a80.gz` to the reference baseline directory as
`baseline.a80`, and retain its other historical files. The copied PC helpers use
adjustable repository/baseline arguments; their historical manifest remains
original. Current and historical source/ASM/binary equality is recorded in
`history-current-comparison.json`. The original PC instrumentation overlay is
reference-compiler specific and is not claimed as a current-parent overlay.
