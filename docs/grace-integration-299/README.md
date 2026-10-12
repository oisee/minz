# SQL result filter: допуск колонок и LIMIT

Root `9ec0f503`: real SQLite regression **PASS 0,005 с**, весь пакет `cmd/mzv` **PASS 0,004 с**. SQL filter использует bound value и квалифицированное имя `_minz_filtered."column"`: неизвестная колонка вызывает ошибку. Отдельная нагрузка base `LIMIT 1` и фильтр `LH` подтверждает фильтрацию уже ограниченного результата — ноль строк. Проверяются Scan и rows.Err.

Первичное исправление ORDER BY — `ddcbbaec`, followup — `9ec0f503`. Это runtime исправление MZV, не новый Grace выигрыш или полный screen_report proof. ALV исторический manifest относится к первому SQL helper: его старые bytes сохранены на шаре в source-delta-866aeb2d-to-ddcbbaec и alv-report-main. Новая версия не подменяет исторический manifest.

Whole-main ALV/selection/SQLite root gate и каталог146/194 остаются [отдельным checkpoint298](../grace-integration-298/README.md). Commander/report/PFCCO arena продолжаются независимо.
