# Original SHA-labelled split helpers: independent native arithmetic evidence

Exact original `examples/nanz/sha256.nanz` SHA256
`6a19378d623d83ad05f99b271b19daa94013f1ea069318e7210f230fd5248540`
was frozen before testing and is unchanged. It contains 15 split-u16 helpers and
wrappers, not a complete SHA256 implementation. Source comments promising full
hash/basic vectors remain historical source text, not evidence of compression,
padding, digest or block processing. The only added source is an explicit
read-only observer runtime declaration/function; no algorithm is rewritten.

Independent host models use unsigned-width arithmetic (Book v7 chapter 3),
32-bit addition modulo 2^32, ordinary logical right shift returning zero for
counts >=32, and FIPS PUB180-4 §4.1.2 Boolean definitions Ch/Maj. These Go models
never execute compiler IR or reuse the source's split-byte algorithms.

69,480 original-function calls plus one separate RAM observer per build:

- All 65,536 `not16` inputs.
- 369 cases each of `add32_hi/lo`, carry/overflow, boundary words and 32 walking bits.
- 625 cases each of `xor16/and16`, boundary and 16 walking-bit words.
- 729 triples each of original composed `ch_lo/maj_lo`, including nested calls.
- 246 cases each of `shr32_hi/lo`: 41 values × counts 0,1,7,16,31,32.
- All six original wrappers and six declared `via mir2` assertions.

Raw VM, final optimized VM, and actual assembled native ordinary/Grace-speed/
Grace-size all agree with the independent expectations. Complete guarded RAM
stays unchanged, arithmetic calls have no semantic data events, the separate
observer has its exact read, and native harness checks restored SP, return PC,
code and memory authority. All three images are identical: 376 bytes,
13,405,265 T per workload, SHA256
`1284beea5199390f7e5d3b8acaa384b4e030cce63752baaf3b1f4422bd618942`.
Native executed 2,139,052 instructions, within the explicit 10M cumulative bound.

`TestOriginalSHASplitHelpersVMZ80` PASS 6.209 s. Initial final-VM-only 1M budget
exhausted at call56,029; raised to 10M, matching the declared workload, rather
than capping or reducing cases. This was a harness limit, not a miscompile.
No production/source fix was needed. The three modes are direct execution proof,
not training/heldout selection or a measured optimization improvement.

Catalog exact-path evidence now marks this helper subset, retaining its explicit
unimplemented whole-hash requirement. Catalog194 remains unchanged in denominator:
39 reviewed VM/native paths and one separate native-CPM path on this snapshot.
Concurrent agents may contribute additional rows; do not merge by count alone.
Catalog gate PASS1.812 s. No broad full-pipeline/CLI repetition.

All exact source/observer/compiled-source provenance, workloads, independent
results, raw/native full observations, final MIR and ASM/BIN are retained in
`grace-sha-split-artifacts`, with large JSON losslessly gzip-compressed and a
SHA256 manifest. These are standalone helper models, not cryptographic hash test
vectors or an endorsement of the source's performance estimates.
