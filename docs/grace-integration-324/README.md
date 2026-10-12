# Закрыто переполнение второй ZX-версии ZSQL

Обе программы теперь используют явные 61-байтовые массивы для лимита 60 символов и NUL. Все длины 0..61 в трёх режимах проходят: 372 native вызова с проверкой screen/full RAM/return. Полный zsql_zx_real SQL main также повторён. [Отчёт](../../reports/zsql-zx-embedded-capacity/README.md).

Категории 194 путей не изменены: full zsql_zx main всё ещё ждёт аудита, helper proof не повышает его до полного native допуска. Обновлены точные source/evidence SHA. Полный pipeline94a41120 ещё исполняется.
