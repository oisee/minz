# Wide native, контракты и проверенные локальные таблицы

Интегрированы `6be9d4d4` (таблицы), `e66b02a7` (wide caller/memory) и `cb587fe4` (native PFCCO workload). На объединённой ветке полные MIR2, VIR и z80spec прошли за 8.423, 0.058 и 0.006 s. Проверки u32 production, API option и HashMap heldout прошли за 3.224 s. Все cmd/pkg/internal/tests проходят compile-only.

Проверены 672 случая памяти VM/Z80 для u24/u32, разных pointer/value homes, shadow banks и wrap на ffff; 32 случая CALL с clobber обоих банков и независимым oracle по модулю 2^32. Реальная Nanz-программа с globals, CALL, add/store проходит. Snapshot certificates memory-host пока ограничены 1/2 байтами; это не широкая сертификация.

В PFCCO workload два настоящих CALL, первый результат жив через второй. Независимая модель чувствительна к порядку и переполнению globals. Без оптимизации контрактов: 8857 T / 52 B; greedy и PBQP: 7888 T / 52 B. Сохранён первый кандидат без выигрыша: 6474 T / 68 B во всех режимах. Независимый Python-аудит повторно прошёл.

Checked local keys связывают descriptor, operands, hints, clobbers, ABI, physical overlaps и cost model. Назначение повторно допускается, заявленная стоимость пересчитывается. Исправлена потеря word return перед RET. Пример: 95 → 57 T, 10 → 7 B, HL=1234 и SP=f000. **Обычный production pipeline пока не вызывает VIR/table lookup**; практическая интеграция локального кандидата впереди. Это не доказательство глобального оптимума или выигрыша всего корпуса.

В showcase теперь 193 случая: добавлен pfcco_weighted_packet. Результат с включёнными исключениями: 192 PASS, 1 FAIL, 0 SKIP. ZVM отвергается ясной native capacity причиной: story занимает 131072 B, предыдущие globals — ещё 18377 B. Исходник заявляет запуск через mzv -H; native работа Zork не подтверждена. Полный pipeline на cb587fe4 запущен отдельно; результат пока неизвестен.

Все 77 новых файлов скопированы на шару и проверены SHA256. Пакеты: docs/grace-vir-checked-local-tables, docs/grace-z80-u32-spills и docs/grace-pfcco-native.
