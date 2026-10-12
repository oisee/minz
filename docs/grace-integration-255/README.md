# Корпус после Tetris, CASE и ZX reports

194 точных пути сохранены. Pascal CASE/REPEAT теперь имеет native CP/M main proof; исходный SAP и исправленный hello_zx — bounded native ASM/device whole-main. Tetris CP/M/TUI получили независимые12entry proofs, но игровые main/device остаются непроверенными, категории не повышены.

133 independent/bounded VM/native, семь native CP/M, одиннадцать native ASM и два native SMC. Остальные41: 15inline ASM,18external runtime,5Agon/MOS,2pending,1host-only. Native device proof ограничен frozen scenario, не всем физическим устройством.

Focused registry PASS2,927с. Два исходных failed registry logs сохранены: источник oracle test изменился после reviewed factoring Pascal helper и сохранения native проверок при честном tablefallback/tokenizercost. Обновлены только эти проверенные test SHA, source/prospective workload freezes не подменены.
