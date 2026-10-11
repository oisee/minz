# Initial repair source review: b989352c

Two concrete blockers were sent to the owner before native measurement. No CPU or source mutation was performed by this reviewer.

Generated counted-print binding names use lowerer.fresh, which only increments a label sequence. It does not reserve existing source globals/locals. Since Assign lowering prioritizes a matching global, source global `__print_ptr_1` or `__print_count_2` can capture the generated pointer/count assignments. A hygienic reserved-name helper or direct MIR SSA implementation is required, with a valid collision regression.

Counted S/L printing emits ordinary print.char calls. The real VM builtin currently uses `fmt.Printf("%c", rune(byte))`, reencoding UTF8 bytes above127. Printing literal `é` should emitC3A9, while two char calls currently emitC383C2A9. The present parser output test replaces this host and therefore cannot prove real builtin stdout bytes. Fix byte-exact print.char or provide another byte-correct normal path; add capture of actual VM stdout, including embeddedNUL and nonASCII payload. Native OUT remains raw-byte output.

VM symbol storage now preservesS/L/C kinds and length bounds, dedup identity and empty encoding; emitter validates kinds/limits and removes empty trailing commas. Frontend directliteral representation dispatch and ordinary counted loops preserve pointer-once evaluation and zero-body data-load avoidance by construction. Existing unknown-pointer @print stays CString. The code does not modify originalSelfLanz source. These points are otherwise consistent with the frozen oracle, pending actual allocated machine tests and final hygiene fixes.

Existing focused storage/rawVM tests are useful but do not establish actualZ80 output/liveness or absence of extra native reads. UTF8 parser rejection tests measure byte lengths, not stdout correctness. Representation maximum65535 is a format-boundary test, not a promise that a full native64K executable fits.
