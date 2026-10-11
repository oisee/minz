# Concrete draft findings

Read-only initial memory_host_cli.go snapshot SHA recorded separately. Owner notified; fixes pending. No final admission claim until corrected source reviewed.

1. Workload decoding invokes decodeHIRProfileJSON, whose duplicate-key token walk + DisallowUnknownFields accepts case-insensitive Go struct-field aliases. For example an otherwise valid workload with `Schema` replacing `schema` decodes to the same value. Missing explicit zero fields likewise normalize. This differs from the claimed strict exact closed-host serialized schema. Require canonical exact JSON key/type/tree validation after typed decode, using numbers without float rounding; respect documented omitempty rather than arbitrarily requiring fields.

2. Nanz.ParseWithOpts permits imported source files and source-site identity includes SourceUnits, but validateMemoryHostCLI checks only top-level source/workload/profile against all outputs. Thus an outputFile or artifact name may refer to an imported dependency and overwrite it after successful measurement. Protect all parsed dependency input paths, or explicitly refuse imports for the first CLI contract. Top-level source/hardlink/symlink checks themselves are materially stronger than old generic CLI checks.

Direct output binding is correct by inspection: selected.a80 and selected.bin are exactly selected Candidate.Evidence.Target.Assembly/Binary, no recompile. Decision records typed selection digest and selected BIN SHA/cost/size; typed digest must be independently re-marshaled in Go when auditing output. Baseline includes fixed bounded-workload adapter/layout; flag help correctly names bounded native artifacts. Main default compilation path only gains opt-in dispatch and registration; no native runtime/codegen permission changes.

No CPU run or parent/owner edits. Final build/output/evidence audit remains pending after owner corrections and source freeze.
