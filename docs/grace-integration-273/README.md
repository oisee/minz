# SQLite-мост: native контрпример физического ABI

После read-only allocation audit выполнен настоящий CALL исходных sqlite_exec и sqlite_query. SQL pointer по фактическому allocation находится в named spill slot, db=HL. DE содержит независимый sentinel WRONG. Ожидание: отправка CREATE TABLE t(x) и NUL через порт45; фактически обе функции отправили WRONG и NUL во всех трёх режимах, затем вернулись с правильным PC/SP.

Это воспроизведённое несоответствие source ASM contract: тела читают DE, а formal sql не закреплён в DE. Source, compiled-module probe, exact actual/expected payload и SHA заморожены до исправления. Oracle задан до исполнения. Проба не заменяет SQL engine/полный main и не меняет source/allocator placements. Следующий шаг — явные physical input annotations и независимый повтор, включая прочие ASM API моста.
