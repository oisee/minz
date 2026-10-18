The unchanged whole report `i32-report.abap` is an independent four-byte integer
oracle for the native Agon runtime. `i32-report.json` was generated with the
existing `research/native-abap/itab-reference.mjs` runner using Node22.23.3 and
`@abaplint/transpiler` / `@abaplint/runtime`2.14.8. No compiler-generated result is
used as an expected value. The fixture records the exact source SHA256, complete
report output and final scalar values. The VM regression and native MOS evidence
execute the same source, including its FORM invocation and signed boundaries.
