# Независимый аудит сохранённых native результатов

Исправленный isolated harness ed973641; actual82620 PASS3.593. Проверка не запускает VM/Z80 повторно. Источник SelfLanz неизменён, все три raw/native профиля имеют один собственный compiler context dfdf728..., отличный от production dba. Original BIN/ASM/MIR/allocation/sidecar/frame, per-call T/steps и состояния точно равны сохранённому production dba baseline.

| Вариант | T модели Z80 | Байты | Шаги | Изменённые сравнения |
|---|---:|---:|---:|---|
| Original | 1135370 | 2000 | 109829 | 0 |
| Compare | 1132449 | 2001 | 109966 | intern_region HL < BC |
| Reuse + compare | 1096139 | 1998 | 108955 | intern_region HL < BC; cur DE < HL |

Compare уменьшает суммарный T на 2921 (0.2573%), увеличивает BIN на один байт. Совместный частный эксперимент уменьшает T на 39231 (3.4553%) и BIN на два байта. Это измерения одного workload из 30 вызовов, не универсальная прибыльность и не per-PC ledger. Ранее reuse без compare был отрицательным по скорости; он не включён в default pipeline.

Для compare-only final MIR и allocation byte/hash неизменны. Все 73 descriptor roots/byte widths/directions совпадают; PC и символические адреса переезжают из-за изменения размера кода. Собственные opcode bytes сверены со своим BIN и global addresses. В совместном варианте 72 записи после удаления ровно одного повторного pos load. Call frames отсутствуют во всех трёх. Typed Go audit проверяет SHA без Python reserialization. Полная независимая модель подтверждает результаты, 10 globals, три RAM regions и все 3017 ordered peek/poke events каждого варианта.

Единственное compare-only изменение исполнения находится в intern_region loop_head1: PUSH HL / OR A / SBC HL,BC / POP HL заменены сравнениями H:B, затем L:C при равных старших байтах. HL/BC сохраняются, исходный JRS NC остаётся. В совместном варианте cur сравнивает D:H, затем E:L, сохраняя DE/HL и прежний JRS C. Остальные ASM различия: перенумерация trampoline labels, compiler reg/spill labels после одного MIR Move, relocations. Последовательности compare сами не читают и не пишут память.

Gate ограничивает Ult/Uge, U16 materialized ordinary distinct HL/DE/BC, future-dead A/AF, отсутствие живых pending acc/flag и потребность старого SBC в сохранении живой пары. C и Z соответствуют unsigned word compare, другие биты F не обещаны. Эмиттер инвалидирует A aliases, сохраняет pair caches, выставляет pending condition без swapped/two-test markers. Это подтверждено actual boundary/predicate consumer tests и direct cache/guard tests с реальной cross-block liveness. Исторический handcrafted fixture failure и stale-base original refusal сохранены; core не менялся для их обхода.

Новых default production hooks/оптимизаций не включено. Native harness private globals и MIR reuse helper должны остаться экспериментальными. Broad safety tests, policy выбора по speed/size и полный production gate требуются до любого default включения; текущий результат не сертифицирует transformed HIR и не переносит счётчики циклов.
