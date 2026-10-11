# Фактическая стоимость PC и функций Self-Lanz

Два final прогона прежних 30 persistent вызовов выполняются в закрытом native
runner. Неизменённые actual бинарники совпали с сохранёнными CLI evidence:
original **1221494 T / 127538 шагов**, обе оптимизации **1147025 T / 119303 шага**.
Совпадают ASM/BIN, per-call T/steps, результаты, globals/resources/events и
native sidecar/frame/allocation/MIR hashes. Диагностический процесс имеет
собственную executable identity; его профиль не выдаётся за профиль CLI 1bd955.

| Физический диапазон | Original T | Both T |
| --- | ---: | ---: |
| intern_region | 334605 | 263235 |
| cur | 239391 | 239391 |
| parse_sexpr | 236919 | 236919 |
| make_node | 141414 | 141414 |
| is_atom_char | 120219 | 120219 |
| region_match | 51575 | 48476 |
| adapter:peek | 35292 | 35292 |
| adapter:poke | 15997 | 15997 |

Оставшиеся диапазоны есть в полных таблицах. Все **74469 T** экономии
распределены между intern_region (**71370 T**) и region_match (**3099 T**).
Остальные физические диапазоны имеют прежнюю стоимость.

Сборщик записывает pre-step instruction bytes и фактическую T-state delta
только после успешной проверки native instruction/fetch/permissions/frame.
Каждый PC соответствует точной инструкции assembled listing/BIN. Суммы
проверены по каждому вызову, всему workload, владельцам и горячим PC.
Hook остаётся только в изолированной экспериментальной ветке и не включён
в production компилятор или публичную схему evidence.

Assembler listing не содержит standalone labels/EQU. Поэтому сырые журналы
сохранены с исходным owner `unowned`; они не переписаны. Отдельная offline
projection восстанавливает physical labels из того же exact ASM и symbol
таблицы, проверяя границы и bytes инструкций. Peek/poke учитываются как
адаптеры, `node_tag EQU peek` — alias metadata без отдельного тела.
Диапазоны идут от emitting entry до следующего entry; последний заканчивается
на последней executable instruction. Внутренние compiler helpers разделяют
физический диапазон. Это **exclusive physical cost**, не inclusive call-chain
cost и не доказательство source-loop lineage. Нераспределённых T в final
projection нет. Значения относятся к Z80 emulator model, не hardware timing.

Два ранних original-only запуска отклонены из-за другого относительного
имени source unit в первой строке MIR dump; BIN/cost/state и остальные хеши
совпали. Логи и payload сохранены. Final factory использует точное логическое
имя source unit CLI; хеши сравниваются напрямую, без нормализации.

[Артефакты и ограничения](grace-memory-host-pc-artifacts/diagnostic/evidence/README.md) ·
[Независимый аудит](grace-memory-host-pc-artifacts/independent-oracle/FINAL-AUDIT.md) ·
[SHA256 манифест](grace-memory-host-pc-artifacts/manifest.json).

Перенесённые gzip payloads проверяются без повторного CPU:

```sh
python3 -B docs/grace-memory-host-pc-artifacts/diagnostic/evidence/independent-oracle-review/audit_pc_pack.py docs/grace-memory-host-pc-artifacts/diagnostic/evidence/final docs/grace-memory-host-pc-artifacts/diagnostic/evidence/source.nanz /tmp/pc-audit.json
```

Следующий измеримый кандидат — exact symbol-bound u16 global load в DE/BC.
Сейчас byte-pair sequence стоит 36 T, direct ED word load — 20 T. На двух
горячих cur loads счётчики дают оценку 46608 T; это не measured whole-caller
выигрыш. Отдельный backend prototype должен проверить live registers,
AF/flags, exact global address и порядок low/high byte accesses до интеграции.
