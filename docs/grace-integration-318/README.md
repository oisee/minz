# Два исходных SQLite main в native корпусе

sqlite_test и sap_mara_cpm теперь имеют проверку полного main с исходными портовыми SQL/TUI ASM, настоящей SQLite и двумя BDOS clobber-контрактами. Каталог переводит ровно эти два пути из inline ASM audit в native CP/M runtime category. Independent/bounded VM/Z80 остаётся153/194, native CP/M runtime9, inline ASM audit7. Это не расширяет доказательство до raw/final VM или физического CP/M.

SAP MARA: 360 точных stdout bytes,31 команд,12 значений колонок, JOIN/filter/order, все receive writes/хвосты, full RAM иPC/SP. Во всех режимах2820B/78892T. SQLite test после backend897e7646:1963B/26125T. Joint native PASS0,408с. Каталог PASS2,509с. [SAP отчёт](../../reports/sap-mara-port-main/README.md) · [SQLite отчёт](../../reports/sqlite-import-main/README.md).

Полный pipeline94a41120 ещё исполняется; текущий backend/MIR2 и новые main проверки имеют отдельные результаты, не выдаются за PASS того полного executable.
