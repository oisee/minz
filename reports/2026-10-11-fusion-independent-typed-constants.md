# Независимый аудит byte→word fusion: typed constants

Base `708e77c5`, только isolated worktree. Найден действительный counterexample в valid SSA MIR: low pointer = p + Const(255,TyU8), high pointer = p + Const(256,TyU8). VM обрезает вторую константу до 0, значит исходные byte loads читают p+255 и p+0. Matcher использовал исходное Imm=256 и ошибочно считал адреса соседними. Raw VM давала 0x1234, fused VM — 0x5634. `Verify` принимал изменённую программу по той же ошибке. Before-log: `fusion-typed-offset-counterexample.txt`.

Исправление использует MIR OpConst stored bits в трёх местах fusion: constant predicates (shift8/mask255), pointer offset proof и verifier retained-address proof. Тайпинговые требования и effect barriers не ослаблены. Отдельно та же ошибка обнаружена и исправлена в `MemoryHostGlobalOrigins`: и обычный OpConst transfer, и evaluator retained SequentialAddressDefs должны использовать declared-width значение, а не raw Imm.

## Узкая независимая проверка

- TyU8 Const256 больше не допускает fusion несоседних p+255/p+0; результат остаётся исходным.
- TyU16 Const256/65792/-65280 — одинаковые typed bits256: fusion допустим и совпадает с raw oracle. Проверены базы 0/65280/65535 и modulo65536 wrap.
- Forged typed-invalid retained metadata отвергается verifier.
- Shift constant264:TyU8 распознаётся как фактический count8, raw load/store semantics сохраняются.
- MemoryHost origins Const256/257/-255:TyU8 дают offsets0/1/1, а fused retained high-pointer Const65792:TyU16 доказан offset256.
- Исходные SequentialByte tests callback identity, fault-prefix/wrap/no-observer и renumber повторены; CFG predecessor-meet origin negative тоже повторён.

Focused MIR2/HIR PASS 0.014/0.009 s (`fusion-independent-focused.txt`). Полные pipeline/CLI root выполняются на предшествующем checkpoint и не перезапускались этим аудитом. Native noncanonical source constant codegen не измерялся; контрпример — разрешённый typed MIR и его VM/optimizer/verifier семантика. No gain или full corpus PASS не заявлены.

## Остальная read-only оценка

CY language effects не пересекаются matcher: между byte ops допускается только небольшой pure opcode список, Call/Error/ASM/port/store barriers не включены. Fault-prefix VM loop читает/пишет byte0 до byte1 checking, оригинальные *Inst pointers используются для callback identity. Renumber remaps address metadata и origin Src[0]; origin Dst/Src[1] остаются original callback provenance. Этот аудит не утверждает, что вся mutable metadata callback identity является cryptographic authority: допускается только независимое доказательство фактических адресов. Позднейшие generic SSA mutation passes должны сохранять metadata либо отказывать; общий доказанный contract всех downstream rewrites здесь не заявлен.
