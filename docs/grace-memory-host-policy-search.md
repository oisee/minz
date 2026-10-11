# Совместный выбор policy и source transformations

Новый opt-in API `MeasureMemoryHostArenaAppendPolicySearch` измеряет восемь
сочетаний: Legacy / PreserveForSpeed × original / arena-first / append-copy /
оба. Обычный компилятор и fixed-policy CLI сохраняют прежнее поведение.
Результаты относятся к неизменённому `self_lanz_parser.nanz` и указанному
workload, а не к произвольным программам.

До caller factory API принимает две обязательные whole association v2,
проверяет их policy/source/workload и независимо разбирает сохранённые source
bytes. Полный HIR, ABI, origins и metadata сверяются с этим reference. SHA
исходника сам по себе недостаточен. Графы удерживаются живыми на всех48
callbacks; source sharing, поздние мутации и failed candidates дают отказ
без частичного победителя. Каждый fixed-policy selector выполняет fresh
replay и source/native checks. Search сохраняет оба полных selection и
восемь ссылок policy/mask; стоимость берётся из проверенного native payload.

Строго сравниваются фактические T-states или длина BIN. При равенстве основной
стоимости сохраняется более ранний кандидат: Legacy masks0..3, затем
PreserveForSpeed masks0..3. Cross-policy observations сохраняют все поля,
включая Error/Failure; допустимое отличие Steps проверяется отдельно.

## Измерения

Full30 actual Speed search PASS83.79s; ровно48callbacks. Два свежих collection
baseline совпали с прежними public source/native результатами.

| Mask | Legacy T / B | PreserveForSpeed T / B |
| --- | ---: | ---: |
| Original | 1135370 / 2000 | 1132449 / 2001 |
| Arena-first | 1135211 / 2121 | 1132290 / 2122 |
| Append-copy | 1079795 / 1978 | 1076874 / 1979 |
| Оба | 1079636 / 2099 | 1076715 / 2100 |

Actual Speed выбирает PreserveForSpeed + оба преобразования,1076715T/2100B.
Size, вычисленный из сохранённых BIN, выбирает Legacy + append-copy,
1079795T/1978B. Full30 actual GoalSize нового API не выполнялся.

Короткий zero-body контроль PASS113.95s,48callbacks: actual Speed выбирает
PreserveForSpeed + arena-first,566T/2122B. Derived Size выбирает Legacy +
append-copy,663T/1978B. Это отдельный контроль переносимости/search; нулевые
body counters не доказывают dynamic compare activation.

## Проверки и артефакты

Focused admission/valid retained-SHA altered-body PASS7.66s; forcedGC/lifecycle
и late mutations25/48 PASS314.10s; owned-input/actual short GoalSize/tie
PASS118.45s; missing/options preflight PASS4.37s. Первое падение неправильного
fixture и его executable сохранены; исправленный negative сохраняет валидные
HIR origins. Synthetic ranking tests проверяют ordering, а не подлинность
машинных замеров.

Два независимых saved audits и повторная сверка перенесённого пакета прошли:
300full +20short observations, все11 profile counters, память/ресурсы/globals,
ordered events, source/precompile proofs, association bytes/SHA, ASM/BIN,
MIR/allocation/sidecar/call-frame SHA. CPU при аудите не запускался повторно.

[Пакет и manifest](grace-memory-host-policy-search-artifacts/manifest.json)
содержит точные inputs, complete searches, все16кандидатных BIN, ASM/MIR,
логи, independent auditors и четыре retained executables. Большие payloads
сохранены как gzip; auditor читает точные восстановленные bytes. Final
executable SHA2955420de33c6ce620f2298b0712ab537f0b66cd7fc540c545d344bff1b55966.
[Полный audit](grace-memory-host-policy-search-artifacts/relocated-full-audit.json)
и [short audit](grace-memory-host-policy-search-artifacts/relocated-short-audit.json).

API перенесён `44739f77`, test-only external frontend packages `be43274d`
устраняют integration import cycles без изменения production source gate.
HIR/MIR/C89/CLI/Nanz и affected frontend tests PASS; showcase31same0new.
Общий pipeline TERMINAL PASS (exit0), полный лог и exactexecutable сохранены
в parent-gates пакета. API-этап закрыт; overall цель продолжается.
Следующий шаг после gates — отдельный явный search CLI с paired collect/replay
и exactselectedASM/BIN.
