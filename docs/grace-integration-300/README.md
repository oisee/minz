# Commander: целый main в общем checkpoint

Root `0d599b34`: совместные Commander, selection demo, ALV и SQLite demo **PASS 2,358 с**. Commander исходник не менялся. Сценарий Down/F5/F3/Up/Esc исполняет пять кадров и все три функции. Независимые 204 terminal events, 2729 ANSI bytes и конечные globals совпали в raw/final VM и собранном native во всех трёх режимах.

Native использует явные test-owned typed TUI hosts с register-clobber stress; фактические caller/formal ABI сохранены. Проверены память, код, строки и возврат main к PC7003/SPf000. Ordinary 1572 байта/16407 T, оба Grace 1574 байта/16353 T: на 54 такта быстрее за 2 байта. Это наблюдение для конечного сценария, не выигрыш Size selector или аппаратная проверка. [Полный отчёт](../../reports/2026-10-12-tui-commander-main-native.md).

Manifest payload и отдельный сохранённый executable находятся на шаре. Каталог добавляет только exact tui_commander; proof на generated L3 не переносится. Screen report и PFCCO arena пока в работе.

Root каталог **PASS 2,781 с**: 147 из 194 точных путей с независимыми полными или ограниченными VM/native проверками.
