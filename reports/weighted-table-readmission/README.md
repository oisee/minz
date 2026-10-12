# Сохранённая таблица weighted: повторный допуск без solver

ReadmitCheckedLocalTables использует saved docs/grace-real-leaf-local-tables/caller-table.json как bounded пул предложений. Текущий MIR exact source examples/nanz/06_pbqp_weighted.nanz с явным caller harness подготовлен заново; назначение weighted снова допускается и выбирается обычным Size selector. Caller содержит CALL и остаётся на обычном backend.

Полный модуль44→20байт. Все256caller inputs проверены в MIR VM и собранном Z80 по независимой формуле byte(11*x+1), включая восстановленный SP и фактический ABI. Сумма native тактов60928→36352 (238→142на вызов). Сохранение живого x черезCALL включено. PASS0,451с. Внешний solver не требуется и тест не пропускается при его отсутствии.

Это повтор существующей пользы на том же corpus helper, не новое семейство или prospective heldout. Saved ASM/BIN и source находятся в docs/grace-real-leaf-local-tables; новый тест заново строит baseline/selected для текущего compiler. Полные pipeline94a41120/dd100284 продолжаются; directed PASS не заменяет их результат.
