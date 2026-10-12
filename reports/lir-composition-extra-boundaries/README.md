# Composition fixtures: additional argument boundaries

Unchanged source/model fixtures a1/a2/a4/a8 from the transferred LIR corpus. First two byte arguments cover all65536 pairs; c uses frozen0/1/127/128/254/255; a2 additionally covers all36 selected c/d pairs. Total54 domains and3538944 assembled-Z80 calls. Every domain asserts65536 actual input invocations; mathematical models remain modulo256.

Production ordinary pipeline and allocation are used, followed by actual assembled CALL/RET. The existing value judge checks result and bounded termination; this extension is not a fullRAM/SP/flag oracle, all3/4-argument Cartesian, Grace/table comparison or performance result. No production behavior changed. Existing original test body and six fixtures remain unchanged.

Command: GOCACHE=/tmp/minz-grace-gocache go test ./pkg/hir -run '^TestNativeCompositionAdditionalArgumentBoundaries$' -count=1 -timeout=15m -v. ToolchainGo1.26.8. New test: minzc/pkg/hir/native_composition_boundary_test.go. Log retained; no broader fullgate restart. This closes the specifically identified additional-argument value-coverage gap in the LIR transfer review.
