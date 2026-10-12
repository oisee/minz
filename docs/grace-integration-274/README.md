# SQLite ASM: явные физические входы

После сохранённого native контрпримера273 входы ASM библиотеки закреплены в используемых ими регистрах: word handles/pointers HL, SQL pointer DE, column A. Изменены только param annotations, не ASM тела/портовый протокол/allocator. В частности sqlite_exec/query больше не читают посторонний DE при formal SQL spill.

Native root TestSQLiteASMInputABI PASS0,193с: полный исходный sql_test6 разбирается и компилируется в трёх режимах, затем настоящий CALL двух entry проверяет строки длины0/1/17/255 и handles1/255/1234h. Ordered sent bytes/NUL, command8/3, handle lowbyte, оба байта адреса OUT, статус/return value, точный returnPC/SP и неизменность RAM вне фактического стека проверены. Actual formal allocation должен HL/DE. Сохранён before WRONG и after точный CREATE TABLE t(x).

Это scoped entry proof, не полный SQLite engine/main. _sql_recv_str всё ещё требует NUL/буфер ≤256 согласно протоколу, а другие port API нуждаются в отдельных bounded runtime проверках. Их annotations выводятся из прочитанных ASM тел, не из догадки PFCCO комментариев. Исторические source/evidence SHA не переписываются; новый dependency hash может потребовать явного обновления reviewed corpus evidence после повторов.
