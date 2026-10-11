# Финальный source/admission review policy CLI v2

Проверены frozen source c7c42795 и inventory/checks 6647fe0b; SHA всех четырёх cmd-файлов независимо совпали. Parent/compiler и owner source не изменялись. Новых CPU-прогонов reviewer не запускал.

Материальных оставшихся дыр приёмки в рассмотренном scope не найдено. Новый режим включается при любом Changed новом флаге, включая явно пустой. Пустые Changed legacy directory/profile-флаги также отклоняются. Тесты сохраняют и восстанавливают Changed для всех пяти флагов. Общий workload-флаг разрешён намеренно. При отсутствии новых флагов прежний CLI и ordinary route остаются прежними: diff old runner отсутствует, извлечена только прежняя path/admission проверка.

Фабрика читает один owned source buffer и каждый раз проверяет complete/one-unit/source SHA; cached buffer исключает повторное чтение изменённого исходника. Imports admission ограничен одной единицей. В import-negative тесте unresolved import может быть отклонён самим Parse: это честный тест отказа, а не отдельное доказательство выполнения one-unit guard. Workload/association чтение ограничено; обязательна ровно named policy legacy/preserve-speed. Whole v2 association передаётся строгому loader/replay без преобразования v1 или подмены policy.

Alias validator получает все 27 новых имён файлов и optional -o; каждый input/output сравнивается canonical path и SameFile. Tests проверяют source hardlink, workload symlink, association hardlink и -o alias на selected.bin представительстве. Проверка повторяется непосредственно перед записью. Она не является защитой против параллельной враждебной смены filesystem между проверкой и записью; такого контракта не заявлено.

Selection/association SHA вычисляются по именно сохраняемым pretty-JSON/input bytes. Выбранные ASM/BIN берутся непосредственно из mandatory nested baseline v2, без повторного codegen. Тест collect/replay проверяет SHA обоих документов, typed policy, binary SHA, точные selected ASM/BIN, submitted association byte identity и external -o identity. Wrong policy и malformed/duplicate/case-alias/null/v1 inputs не создают output directory. IO failure может оставить частичные файлы; код это явно указывает, semantic refusal до writes.

Owner terminal gates: collect/replay 47.206 s PASS; admission/path/malformed/import/legacy route 0.101 s PASS; existing old CLI validation/path/workload keys 0.041 s PASS. These are focused short tests, не полный оригинальный 30-call CLI evidence. Label audit node_tag diagnostics в short output не скрыты; native/alias correctness bind остаётся pipeline responsibility и будущим saved-only actual audit.

Release: готово к owner preserved-executable actual CLI slice после root разрешения. Reviewer не подтверждает ещё не выполненные full30 CLI policy mode costs, executable identity или saved candidate payloads.
