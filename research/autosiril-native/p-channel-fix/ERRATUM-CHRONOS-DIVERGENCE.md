# Erratum: CHRONOS-DIVERGENCE.md checkpoint-3 hypothesis

For the parent to append/replace in the VT2 research history — this worktree
does not modify the other worktree's files.

`/tmp/minz-autosiril-vt2-z/research/autosiril-native/vt2/CHRONOS-DIVERGENCE.md`
hypothesized that the native chain's arrange input differed from Ruby's
(implicit "{51,52}" pitch set at channel 1 row 259). That is **false**. The
native chain received the correct dyad `{51,58}`; stage 2 `EVENTS.BIN` is
byte-identical to Ruby `pmod` (all 2,097 groups). The first divergence is in
stage 3 arrange, and its cause is **fixture configuration, not the algorithm
and not the input**: the chain and its Ruby references were generated with
`max_offset` 24 (the `research/autosiril-vm/oracle.rb` default) instead of the
canonical invocation's 6, so `squize_ornament` kept offset 0 at limit 24
(mediana 7) and emitted base note 51 + ornament id 4 (`[0,0,7,7]`), where the
canonical settings clip it and rebase to note 58/ornament 0.

Executed proof (checkpoint-p2, `prove_canonical6.py`): with the canonical
`max_offset 6` baked into a generated arrange source variant, the actual
native chain (probe raw-mir2/final-mir2/z80, standalone `ARRANGE.COM` +
unchanged upstream `ECHO.COM` through mze, and the native mix kernel on all
three engines) equals the actual original Ruby pipeline at **every** stage
boundary — stage2 2,097/0, stage3 2,097/0, stage4 3,215/0, stage5 3,215/0
diffs, zero missing/extra cells, final renderer input rows identical to Ruby
`mmod`. The legacy 24 chain remains valid as its own self-consistent profile
and is preserved unchanged.
