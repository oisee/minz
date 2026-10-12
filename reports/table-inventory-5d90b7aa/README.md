# Где текущие shape-таблицы применимы

Checkpoint `5d90b7aa`: две opt-in проверки инвентаризации и исторического импорта завершились с PASS. Команда: `COMPOSITION_EXPORT=<inventory> INDEXED_IMPORT_EXPORT=<indexed-import> pipeline-5d90b7aa.test -test.run="^(TestCompositionActualInventory|TestIndexedActualHistoricalENRT)$" -test.count=1 -test.timeout=30m -test.v`. Исторические ENRT4/5 прочитаны с `/mnt/safe/z80-compiler/tables/`.

Инвентаризация охватывает 72 Nanz-исходника и 981 запись по функциям. Текущий мост допускает 36 задач. Из 945 отказов 710 требуют формы single-result leaf block; остальные причины перечислены в [result.json](result.json). Это объясняет ограниченное покрытие рабочего selector, а не количество записей исторической базы. Импорт частного quotient для weighted завершается; шесть широких задач отказываются по byte-only, hash3 — по singleton ABI. `read completed` ещё не означает выигрыш или допущенный кандидат.

Отдельная проверка импортирует кандидаты для четырёх программ и собирает выбранный ASM: weighted 36 → 12 байт, function pointers 34 → 34, StrRef 45 → 45, rotate sled 186 → 186. Эти проверки не запускают независимые VM/Z80 оракулы. Такое покрытие для отдельных функций описано в предыдущих пакетах `indexed-coverage-44ba35a3`, `indexed-reuse-current` и `offline-table-gate-604547db`; оно не переносится автоматически на все строки инвентаризации.

Исходники, задачи, таблицы и полный лог сохранены рядом. Основной полный pipeline работает отдельно и ещё не имеет окончательного результата.
