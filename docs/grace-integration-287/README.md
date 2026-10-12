# MIR shared arguments: интеграция

Source1fd3d63f. RenumberRegs теперь отделяет owned slices до изменения IDs. Общие и пересекающиеся CFG аргументы, включая backing arrays между функциями, больше не перенумеровываются повторно. Root minimal regressions PASS0,011с. Независимые original terminal/ABAP main проходят, исходники и frozen ожидания не менялись.

Первый совместный runtime gate FAILED3,504с: SQLite тест требовал старый live MZX SHA, хотя использует собственный SQLite adapter. Исторический provider код остаётся frozen и проверяется как audit snapshot; actual source/stdlib dependencies продолжают проверяться в рабочей ветке. Frozen oracle/manifest не переписаны. SQL команды, stdout и RAM assertions прежние. Native consumer actual MZX проверен отдельными cmd/mzx tests. Общий retained pipeline1f2365be ещё жив; эти scoped gates не заменяют его.

[Source counterexample и полный donor proof](../../reports/terminal-screen-main/REPORT.md). Донорский full MIR PASS90,801с относится к его checkpoint, root full MIR после CP/M integration будет отдельным запуском.

Исправленный joint gate: PASS3,251с, пять runtime families реально исполнены. Root full MIR1d9c76c9 запущен отдельно.
