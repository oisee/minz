# Screen report: фильтр LH и исходный whole main

Root `f1408911`: report/ALV/Commander/L3/SQLite demo совместно **PASS 4,064 с**. Исходный screen_report не менялся. Два сценария: F8/F3 выводит все восемь авиакомпаний; Enter/LH/F8/F3 повторно запрашивает только Lufthansa. Реальная SQLite исполняет seed/query/column/close во всех raw/final VM и native запусках, три режима компиляции.

Независимые frozen SQL rows и terminal/table модель совпали: 503/711 событий, 2202/2766 байт вывода, вся таблица 472 байта включая оставшиеся после nrows-reset хвосты. Реальный generated table/padding ASM исполняется native. Фактический extern ABI, точные host scratch writes, RAM/image и возврат через tail close проверены. Runtime привязан явно в тесте; public unlinked native SQL/CPM не заявляется. [Подробности и сохранённые ошибочные ожидания](../../reports/alv-report-main/REPORT.md).

SQL helper исправлен отдельно, без изменения исходного report. Native cursor adapter здесь внешний MZV API, поскольку source не импортирует tui.render. Historical first-model/format diagnostics сохранены и отделены от настоящего runtime дефекта ORDER BY. Fullpipeline и PFCCO arena ещё в работе.

Root каталог **PASS 2,784 с**: 149 из 194 путей с независимыми полными или ограниченными VM/native проверками.
