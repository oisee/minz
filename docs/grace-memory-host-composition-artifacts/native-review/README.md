# Независимый аудит сохранённых четырёх native-вариантов

Проверены сохранённые payload из `grace-checkpoints/memory-host-arena-append-evidence`, без повторного исполнения compiler VM или Z80. Python-аудит сравнил 30 полных raw/native observations каждого варианта; отдельный последовательный Python-модельный oracle проверил реальные source inputs, все 10 globals, 4096 байт nodes, 8192 байт strings, 4096 байт source и каждый упорядоченный read/write event. Oracle воспроизводит оригинальный SelfLanz и переиспользован из исходного evidence pack; его запуск и сверка здесь независимы от measured Go host oracle. Typed Go-аудит подтвердил четыре SHA sidecar/frame/alloc/MIR/BIN; Python-пересериализация для typed digest не использовалась.

| Mask | Вариант | T states | BIN bytes | Access records | A-byte snapshots |
| --- | --- | ---: | ---: | ---: | ---: |
| 00 | original | 1221494 | 2091 | 90 | 0 |
| 01 | arena | 1218395 | 2212 | 92 | 1 |
| 10 | append | 1150124 | 2076 | 90 | 0 |
| 11 | both | 1147025 | 2197 | 92 | 1 |

Все четыре payload имеют идентичные source SHA, owned workload и compiler/host/context. Saved ASM/BIN/MIR/allocation точно совпадают с embedded полями. Суммы 30 отдельных T-costs равны totals. Изменённые body hashes точно соответствуют mask: только region_match, только intern_region, обе или ни одна; ABI и source-unit digests неизменны. Speed winner — both; size winner — append. Эти выводы относятся к данному конечному workload.

Проверены opcode bytes каждого global/spill descriptor по точному PC, символу и bounded offset/width/direction. Две имеющиеся snapshot-последовательности строго соответствуют прежнему ограниченному ABI: одна AF snapshot, результат A→C шириной 8, 19 точных инструкций и contiguous PCs. Никакого расширения stack/ABI permission не понадобилось. Payload не содержит динамического trace всех обычных PUSH/POP/CALL/RET; его законность обеспечивается закрытым runtime и успешным actual-run владельца, а не выдуманным сохранённым trace.

Существенная граница evidence: это prototype baseline, а не selector certificate. Typed aggregate precompile-proof/choices/source-loop-site-map record не экспортирован. Сохранённый factory source вызывает ValidateMemoryHostArenaAppendChoices для каждого свежего mask перед публичным baseline; construction log проверяет source loop identities body/s1→s4 и body/s5→s7. Отдельные input.hir snapshots экспортируются после исполнения, а payload function maps относятся к измеренному input. Аудит подтверждает точные function-body delta/ABI/context и source factory control flow, но не утверждает существование сериализованного aggregate certificate, inherited counts или post-lowering lineage.

Воспроизведение без CPU replay:

```sh
python3 audit_saved.py /path/to/memory-host-arena-append-evidence
python3 host_oracle.py /path/to/memory-host-arena-append-evidence/both
# Из minzc того же production API checkout; временно скопировать audit_typed.go.txt как audit_typed.go:
go run /path/to/audit_typed.go /path/to/memory-host-arena-append-evidence/both
```

Повторить два последних аудита для original/arena/append. Скрипты поддерживают deterministic gzip payloads. `payload-inventory.json` фиксирует все сохранённые исходные bytes на момент аудита; manifest этого отчёта фиксирует его собственные артефакты. Parent и prototype production не менялись.
