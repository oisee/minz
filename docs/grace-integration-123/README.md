# Wide edge-copy correctness and retained executable replay

Root e597afc4 fixes ordinary u32 loop swap losing shadow halves.
512 source raw/final VM/native cases plus wide operation regression
PASS1.614s. Full MIR2 including30 forced pair/triple/spill/constant native
edge sets and previous overlap/caller tests PASS20.940s. Whole-function
tables still decline CFG/u32; this fixes ordinary production edge movement.

Original temporary popcount evidence executable unavailable is explicitly
documented. New separate evidence has a retained25MB executable, exact
SHA107c9f003624a273f139fede4747ffdfc389ead52c482603ead5cabe69779a42.
Root executes that exact binary from the source-resolving pipeline directory:
PASS, all256 inputs7168T/512B. Newly generated evidence compiler_id matches
the retained executable; no historical packet overwritten.

Three dword source/ASM/BIN and15 retained executable/evidence payload hashes
verified and copied to share. Larger mixedwidth edge audit, error semantics
and sequential-byte fusion still ongoing; overall goal remains active.
