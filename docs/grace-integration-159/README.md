# Интеграция 159: исходный Scheme-feature Lizp пример

Неизменённый `examples/lizp/scheme_test.lizp`: все 11 функций и полные byte-домены их аргументов. Восемь unary functions ×256 и три binary functions ×65 536 = 198,656 CALL на режим, 595,968 суммарно. Независимый host-оракул byte arithmetic/parity/min/max/cond сравнивается с raw VM, final VM и собранным Z80. Book v7 §3.1 и §21.2; исходные assert не служат оракулом.

Три режима PASS 2,944 с. Полная RAM и code-image неизменны, кроме фактически использованных 10 байт стека; spills нет, PC/SP восстановлены. BIN 138 байт во всех режимах, performance gain не заявляется. Это полное покрытие callable-доменов данного исходника; название не означает полноценную реализацию стандарта Scheme.

[Artifacts+SHA](../../reports/lizp-scheme-native/manifest.json). Каталог сохраняет 194 пути: 49 independent +8 bounded =57; pending80. General struct-return ABI ещё в отдельной ветке и требует завершения.
