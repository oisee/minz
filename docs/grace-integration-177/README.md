# Интеграция 177: исходный R5RS showcase и исправление VM

Оракул исходного `scheme_r5rs.lizp` зафиксирован в `fdae8e86` до запуска. Все 13 callable entry прошли raw VM, final VM и настоящий Z80 CALL в ordinary, Grace Size и Grace Speed. В каждом режиме 289 535 вызовов, всего 868 605. Проверены исходный PC возврата, SP, код и память вне фактически использованного стека/spill. Покрытие bounded: исключены fact_loop(255), divisor zero и полный трёхбайтный домен clamp; точные входы указаны в frozen-workload.md. Модуль 172 байта во всех трёх режимах; нового выигрыша Grace на нём не заявляется.

Интегрированы исправление VM OpCallCond и исходный whole-main Lizp-tokenizer (`697c2d7d`), а также parser reference (`5fe88510`). Совместный root gate MIR2 PASS 0,008 с, pipeline PASS 6,344 с включает их проверки и повтор R5RS. Полные CLI/pipeline выполняются из прежних сохранённых бинарников; этот focused результат не заменяет их окончательный результат.

R5RS исходник, raw/final MIR, ASM/BIN, CALL drivers, counts и использованный стек сохранены в reports/lizp-r5rs-bounded с манифестом SHA256; копия на шаре проверена. Parser reference packet: все 51 manifest payload проверены. В tokenizer packet обнаружены два отсутствующих tracked probe payload; агент досохраняет их, полнота этого манифеста до исправления не заявляется.
