# Полный core после условного возврата и Pascal Hello

Checkpoint `76945771`, один запуск `go test ./pkg/hir ./pkg/mir2 ./pkg/nanz ./pkg/vir -count=1 -timeout=30m`, session31401: terminal exit 0. **HIR PASS 37,493 с; MIR2 PASS 71,654 с; Nanz PASS 23,217 с; VIR PASS 2,351 с.**

Включает TermCondRet fix, точный Pascal Hello CP/M допуск и public CLI .pas validation, typed local CP/M provenance и новые original ASM/IRC доказательства. Не включает ещё неготовое signed decimal Pascal исправление или Tetris proof. Это full core, не full pipeline/CLI и не утверждение о всех native приложениях.

Отдельный retained pipeline29df08bb остаётся live; self-parser test тоже выполняется на прежнем executable. Наличие позже завершённого core не меняет identity этих прогонов. Исходный start record239 сохранён.
