# CLI: реальный collect/replay с закреплённой таблицей

Один собранный mz выполнил --hir-profile-auto с --Osize, --hir-grace и точным --checked-local-table-sha256. Измеритель проверил полный 1022-case sum domain raw VM/native Z80 и записал model/evidence/observations. Второй запуск того же executable с --hir-profile, --hir-profile-model и --hir-profile-decisions дал побайтно тот же ASM. Выдача BIN тоже прошла: 50 байт. Выбор Size оставляет исходный sum loop и применяет проверенную leaf таблицу в том же модуле.

Изменённая таблица отвергнута до записи ASM. Таблица теперь включена в проверку алиасов входов: decisions/product/artifact не могут перезаписать её, включая hard links. Новые negative cases и прежние strict JSON/flag validation PASS 0,012 с. Production/source семантика не меняется.

На шаре compiler-reports/cli-profile-table-replay/executed лежат точные команды, source/table/workload, compiler identity внутри profile, model, 1022 rows evidence, observations, decisions, ASM и 50-byte BIN. Это CLI доказательство механизма на одном арифметическом модуле; multi-program heldout результаты остаются отдельными прежними проверками. Runtime CPM и compiler-mode search не включены в этот режим, pure Speed остаётся отдельной целью.
