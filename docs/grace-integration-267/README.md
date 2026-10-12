# Parser reference: независимая сверка после интеграции

После forensic refresh общий числовой эталон обновлён только в тестах. Все 30 входов совпадают с независимой host моделью по результатам, globals, regions и ordered events. Association теперь сверяет полный свежий direct artifact до cost assertion.

Исторический baseline 1296873 T / 2391 B → текущий 1296385 T / 2380 B. Exact ASM/MIR delta локализован в parse_sexpr и объяснён HIR contextual literal widths из 1f87d7b7. Source не менялся; production runtime/admission/selector не менялись. Before FAIL, historical5939 и after evidence сохранены в reports/parser-reference-drift-2026-10-12.

Root три positive пути и factory refusal повторены; terminal log рядом. Full pipeline1f2365be продолжает работу со старым reference и его failure не переписывается в PASS.
