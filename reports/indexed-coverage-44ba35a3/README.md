# Повтор покрытия исторических shape-таблиц на 12 программах

Compiler `44ba35a3`, TestIndexedShapeCorpusCoverage: PASS, 18.392 s. Исторические enriched_4v.enr / enriched_5v.enr прочитаны с /mnt/safe/z80-compiler/tables; direct и quotient кандидаты повторно допущены для текущего MIR/ISA/ABI.

Weighted: модуль 36 → 12 B, helper 150 → 54 T на всех 256 входах. Tiny: 11 → 8 B, triple 34 → 22 T; add и double равны baseline. Это helper-замеры без caller; стоимость caller проверена отдельной матрицей.

StrRef test_eq/test_neq: raw кандидаты 17 → 21 T; selector оставляет прежние 45 B. Low_nibble равен 17 T. Function pointers, hashmap, bool flags, promoted/signed arithmetic, arena, impl и tokenizer не получили допущенных записей. Эти результаты не означают невозможности более общего распределения регистров: покрытие нынешнего ограниченного bridge отражено в coverage.json.

Сохранены исходники, prepared MIR, повторно допущенные table.json, baseline/selected ASM/BIN и кандидатные ASM. Независимые входы и их фактическая область указаны в full.txt. Этот повтор закрывает один opt-in SKIP полного pipeline, не переименовывает старый результат.
