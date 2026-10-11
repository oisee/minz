# Интеграция 161: реальные такты исходного PL/M примера

Повтор полного независимого VM/native gate на `b6990cc4`, с накоплением фактических opcode T-states. Считаем настоящий driver CALL, тело и RET; host-настройка аргументов и HALT не входят. ASM/BIN побайтно совпадают с предыдущим correctness packet160.

| Функция и полный domain | Ordinary T | Grace Size/Speed T |
|---|---:|---:|
| ABS_DIFF, 65 536 byte pairs | 3 637 120 | 3 637 120 |
| FIB, все 256 n | 5 208 960 | 4 453 888 |

FIB −14,50% T; весь модуль 47→38 байт. ABS_DIFF не ускорился. Каждый результат по-прежнему сверяется с raw VM, final VM и независимым host-оракулом, плюс full RAM/code/SP. Это полезность Grace на исходном PL/M примере; автоматический профильный selector и prospective heldout здесь не запускались. Имеющиеся отдельные heldout доказательства остаются в пакетах145/156.

[ASM/BIN, workload/cost scope, фактический gate и SHA256](../../reports/plm-absdiff-fib-costs/manifest.json). Общая цель не закрыта; struct-return ownership в работе.
