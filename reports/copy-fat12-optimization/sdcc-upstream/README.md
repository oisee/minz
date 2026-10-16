# SDCC 4.5 upstream black-box pilot

Official release archive SHA and unmodified upstream source SHAs are in method.json. Two original test files were chosen before measurement: bitwise8reg.c (byte AND/OR under register pressure) and gcc-torture-execute-loop-11.c (descending signed loop filling 199 ints).

The common adapter replaces testfwk ASSERT with memory counters; it does not rewrite either test algorithm. SDCC C99 preprocessing generates one shared frozen TU used byte-for-byte by MinZ default/Size/Grace and SDCC 4.5 Size/Speed. C89 preprocessing rejected // in the original #if lines; that initial error is retained. This is not claimed as strict C89 admission.

Both test entries are executed once per build. Independent expected counter and memory values are checked; the 199-element array is seeded A5 and compared byte-for-byte with little-endian 0..198. PC/SP is checked natively, with raw/final MIR VM checks for MinZ. The timings include the full regression entry and ASSERT adapter; they are not a kernel benchmark or a claim about all upstream tests. No performance averages across cases are taken.

SDCC omits the static array a from the linked NOI export list. The observer recovers its address from object SYM area/offset plus linked DATA base, preserving code and original symbol files. Observer aliases, bounds check and hashes are saved. Initial missing-symbol results remain in initial-summary.json.

Results: 10/10 PASS. bitwise8reg: MinZ default/Size 387 B / 1578 T; Grace 370 / 1503; SDCC Size 159 / 606, Speed 160 / 606. Loop: MinZ all profiles 614 / 124104; SDCC Size 522 / 52467, Speed 522 / 52038. SDCC wins both cases in this pilot.

Useful directions visible in output: avoid widening byte temporaries/bitwise operations; direct byte global loads; retain a byte-compatible allocation under pressure; signed countdown branch via sign bit; normalize repeated address arithmetic. These are candidate compiler improvements, not implemented wins.
