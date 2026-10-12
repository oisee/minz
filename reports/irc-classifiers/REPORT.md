# IRC computational classifiers: original source correction and native proof

Exact original `examples/nanz/irc_client.nanz` was frozen before execution in `source.nanz` (SHA256 `2836abd4f06db6a7ae777459e56c9ad35e42aa2ec87c479125685c7d9cdacc96`). Its plain-string pointer arguments contradicted Book v7 §18.1: length-prefixed SString expressions are different from NUL-terminated CString expressions. Original PING returned Unknown in raw VM. The intentional 27-literal source correction is separately described in SOURCE-CORRECTION.md, with each API contract audited. Current corrected exact source SHA256 is `240ecc56c7b35683c15801f20490f22033837dfafa1e4d991ed1ca26a66d6f53`. No parser algorithm or import was rewritten.

The proof parses and emits the entire corrected module and unchanged production `stdlib/tui/render.nanz` import. It executes three original computational entries plus their original `region_match` dependency. It does not substitute a simplified source or bind the network/terminal runtime. External hooks are defensive denials; native instruction fetch is restricted to these four original function bodies. Every opaque ASM/indirect/out-of-scope call in this closure is rejected. The full module image remains present and guarded.

Independent frozen workloads per ordinary/Grace Size/Grace Speed mode:

- `classify_irc`: 16 protocol lines × 6 offsets (including 255), 96 calls. Model independently implements source's ordered PING/PRIVMSG/JOIN/PART/QUIT/numeric/unknown dispatch, using character bytes and declared enum codes.
- `buf_starts_with`: 5 prefix lengths (0,1,4,127,255) × all 256 first-byte values, 1,280 calls. Includes matching, mismatch, and zero-termination; maximum 255 avoids the u8-index nontermination case of unterminated/256-character prefixes.
- `classify_user_cmd`: exhaustive 65,536 ordered first-two-byte pairs, complete remaining 254 bytes retained as sentinels. Independent slash/j/n/q classifier model.

Total 66,912 original entry calls per mode: **200,736 assembled Z80 calls and 401,472 raw/final VM calls**. Terminal result PASS (11.376 s), including negative observer controls. Results, the full 64 KiB state, code image, public access bounds, and original return PC/SP are checked. Actual private stack scratch and declared spill bytes are the only native-memory exclusions. VM global addresses and native linker addresses are explicitly mapped to symbolic objects; no raw-pointer equality is assumed.

Raw VM must match exact source-model public read multiplicity. Final VM/native may delete repeated immutable ordinary-RAM reads: the full actual trace must remain a source trace subsequence, preserve all distinct byte observations and their first-observation order, and admit no added/hidden reads. Negative controls reject reordered, missing-distinct, added, and exact-raw multiplicity mutations. Full-state guards and explicit write refusal prevent mutable/MMIO reasoning from entering this exemption. Per-mode actual public read counts are retained in summary.json: raw 71,433; final/native 69,955 for Grace modes. This is correctness evidence, not a runtime-performance measurement.

A separate production Z80 bug emerged only after correcting source literals: TermCondRet copied the untaken Then edge's byte over its return constant. The one-line emitter fix and exhaustive physical native regression are in `reports/irc-condret-edge` (donor d3c4f964). The original full helper is the real-source regression.

The network event loop, ports 0x30/0x31, terminal BDOS/keyboard, and full main were not executed. Other 22 corrected runtime literals have API-contract evidence only. This packet proves corrected classifiers, not an unchanged-original success or complete IRC application runtime.
