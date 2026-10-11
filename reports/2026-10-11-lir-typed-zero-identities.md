# Перенос типизированных нулевых identities из LIR в MIR

Перенесена конкретная полезная часть старого `pkg/lir/combine.go`: `x|0`, `0|x`, `x^0`, `0^x`, `x-0`, `x<<0` превращаются в Move; AND с нулём и word MUL с нулём — в Const. Это перенос правил над значениями до allocation в обычный `SimplifyIdentities`, без возвращения LIR runtime. Production commit: `42ecf7e5`, baseline production: `08be2950`; baseline test-only source соответствует нынешнему тесту. Предыдущий read-only аудит: `b5a14d88`, `reports/2026-10-11-lir-transfer-current-useful-gap.md`.

Домен ограничен canonical `u8/i8/u16/i16`, точным совпадением типов обоих операндов, доминирующим zero Const в том же блоке и отсутствием непереносимой metadata. Stored typed bits определяют ноль (например u8 Const256 — ноль). Не трогаем hard constraints. Функции с OpCmp (любого класса), flag параметрами/результатами/block params, opaque ASM, conditional/fallible calls и error exits отклоняются целиком для новых правил. Существующие identities не менялись. Физические Z80 flags не служат оракулом эквивалентности: новые правила работают только с MIR values, а перечисленные flag/CY пути исключены.

Byte MUL0 намеренно остаётся прежним: native XOR A дешевле материализации general-class Const. Первый эксперимент показал +3T на таком leaf; итоговый production rule исключает его. AND0 в итоговом варианте LD A,0 и прежний AND0 стоят одинаково. Смешанный ordinary i8 operand+u8 zero, который создаёт parser, тоже остаётся без rewrite — это не canonical same-type MIR.

## Проверка и результат

Frozen `source.nanz` — **новая проверочная программа**, 24 unary identities плюс два branch/zero control leaf; это не изменённая исходная прикладная программа. SHA256 `ff2ccb72f8c85e70b90318a84838c1326396f3ece366c80cd50a41f3db31118b`. Bounded поиск в examples/nanz и reports/showcase-src не нашёл исходного literal-zero consumer; прикладного corpus выигрыша здесь не заявляем.

| Режим | Полный BIN до → после | Workload T до → после | CALL на прогон |
|---|---:|---:|---:|
| Ordinary | 175 → 86 B | 158 812 → 145 384 | 3 752 |
| Grace Size | 175 → 86 B | 158 812 → 145 384 | 3 752 |
| Grace Speed | 175 → 86 B | 158 812 → 145 384 | 3 752 |

Ни один entry в итоговой нагрузке не стал медленнее. Размер −50,86%, такты −8,45% относятся только к этой программе/нагрузке и этим options. Costs включают настоящий CALL, return и ABI moves. Независимый native oracle проверяет все 256 byte values и 14 явных word boundaries; raw/final VM совпадают. Два controls проверяются для всех 256 значений, включая x=0 и x=128. CALL останавливается только на исходном PC с восстановленным SP; все 64KiB сравниваются кроме четырёх driver bytes и наблюдённой stack области, profiler запрещает прочие записи.

Unit MIR models перебирают все 256/65536 stored values для разрешённых typed identities и commutative orientation. Негативы: hard producer/result, typed mismatch, wide/bool, non-dominating definition, wrong SUB orientation, nonzero, metadata, comparison/flag/CY/ASM/conditional-call contexts. Full MIR2 PASS 64.813s, focused unit PASS 0.617s, before и after retained pipeline executable native test PASS. Полные CLI/pipeline gates принадлежат root и здесь не повторялись.

## Воспроизведение

`before-pipeline.test.xz` и `after-pipeline.test.xz` сохраняют точные проверяющие executables; SHA исходного executable записан в `executable-identities.json`. Распаковать `xz -dk …`; запустить из repo root с `ZERO_IDENTITY_EXPORT=/new/empty/path ./…-pipeline.test -test.run '^TestTypedZeroIdentitiesNative$' -test.count=1 -test.v`. All source/ASM/BIN/MIR/options/results/logs сохранены рядом; `comparison.json` содержит per-entry costs. ZIP не используется: это настоящие XZ compressed executables.

Это закрывает bounded value-rule gap, но не доказывает полноту всех ISA constraints, оптимальность allocation, перенос unsafe historical peepholes или полезность на нескольких реальных программах. Shape proposals и текущая checked admission не изменялись.
