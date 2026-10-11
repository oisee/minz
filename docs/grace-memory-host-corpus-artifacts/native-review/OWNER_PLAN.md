# Persistent SelfLanz workload corpus

Base production b770f3be. Source and closed CLI/runtime are unchanged; use the authoritative saved executable1bd95516... with fresh internally collected profiles. No parent edits. New artifacts/workload generation and independent oracles only.

Proposed corpora: short symbol reuse, long atom reuse with early/late mismatches, mixed nested S-expressions with whitespace/comments/numbers, and mostly fresh symbols plus deliberate warm/reuse calls. Reset source bytes, parserpos and nodebump per call, retain string arena/strbump across each corpus. Atoms stay <=255bytes, source <=4096, nodes <=4096, cumulative interned strings <=8192. Exact original parser behavior governs interning and modular number arithmetic.

Independent workloaddriven source-semantics model must verify every result, ten globals, complete node/string/source regions and ordered byte read/write effects. Validate corpus capacity and both exact source sites observed before any native measurement; no output/cost/winner assumptions. Then measure allfouractual masks for speed/size under identical closed executable/source/workload identity and preserve profiles/typedproof/nativeASM/BIN/costs. Do not infer costs additively or extrapolate from the old30calls. Goal is robustness and crossover evidence, not a compilerchange.
