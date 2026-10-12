# Полный MIR2 после CP/M и shared-argument fix

Checkpoint1d9c76c9, весь pkg/mir2 PASS **85,902 с**. Включены publicCPM global-store guards/canonical ABI и shared/overlapping/cross-function slice detachment. Последующие изменения каталога не меняли MIR production. Общий pipeline1f2365be всё ещё отдельный старый retained executable; его числовые parser-reference failures сохранены.

PFCCO prospective heldout и два pinned ABI/native tests повторно PASS (joint1,550с); speed heldout5568T/52B, Size6252T/52B. Два table-теста в этом запуске SKIP из-за отсутствия export/solver env; не считаются проверенными. Повтор с явным offline Z3 tools path запущен. Runtime compiler внешним Z3 не дополнен.

Offline table + PFCCO/Grace combination repeat действительно исполнен: PASS8,797с; corpus audit2,67с. См. tables-final.txt; первоначальные skips сохранены. Это повтор существующих форм/нагрузок, не новое prospective исследование. Solver остаётся offline.
