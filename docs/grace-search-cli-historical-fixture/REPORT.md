# Historical search CLI fixture is bound to its immutable source

The full CLI run at checkpoint 37475dff failed only
`TestSearchCLICollectAndPairedReplay`: the test read the current showcase
`examples/nanz/self_lanz_parser.nanz`, while this closed eight-candidate search
admits exactly the original source bytes.

The failure reproduces on 68f9170f in 0.080 seconds. Current showcase SHA256 is
`763d9f369e3bbbdbcb84dee2627aff5ee90e57d99ad8d0f69e7db0c8325a6b51`;
the historical source admitted by production has SHA256
`0e92689c6bb4a92fd681c9bc70d5d50549631a0fdd669cd026f723cf4e197526`.

The test now reads the already preserved immutable source from
`docs/grace-parser-prospective-heldout/artifacts/source.nanz`, and asserts that
literal historical SHA before writing the temporary test source. Only this
fixture changes. Production exact-source admission, selector/native oracles,
selected ASM/BIN bindings and actual Speed/Size checks remain intact. The evolving
whole-main example has separate current-source tests; it is not relabelled as the
old experiment.

`before.txt` retains the reproduced failure. The focused post-fix run contains
collect, paired association replay, Speed/Size selection and exact native output
checks, together with admission and unknown-source rejection. No full CLI suite
or unrelated pipeline suite is rerun for this isolated fixture change.

Post-fix terminal result: **PASS, 245.382 seconds**. Collect Speed chooses
policy1/mask1 (701 T / 2 486 B); paired replay Size chooses policy0/mask2
(796 T / 2 344 B). All previously existing per-candidate equality, source/association
hash binding, selected ASM/BIN and actual Size objective checks pass. The longer
runtime is the unchanged eight-candidate collect plus paired replay, not a broadened
suite. `after.txt` contains the terminal output.
