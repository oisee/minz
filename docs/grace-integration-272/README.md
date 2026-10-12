# Свежий showcase и SQLite ABI diagnostic

Sourcee23bde5a: полный showcase compile/assemble193PASS/1capacitySKIP,11,644с. Это не runtime proof всех исходников. Старый pipeline1f2365be сообщил третий аналогичный parser referenceFAIL; все три уже сверены отдельно в пакете267, исходный gate продолжает работу.

Read-only original sql_test6 audit в трёх режимах подтвердил конкретное ABI несоответствие: sqlite_exec/query ASM предполагает db=HL/sql=DE, но actual allocations дают db=HL/sql=LocMem. Наличие asm(in...) не доказывает физическое DE без явного contract. Диагностический export и исходный scanner program сохранены рядом; native counterexample нужен до правки. Это не новая заявленная compiler miscompile, пока реальный CALL/effects не воспроизведён.
