# Полный minimal IRC main

Ожидания зафиксированы до исполнения в c3cdbdbd. Неизменённый после двух CString исправлений main получает ровно 500 ненулевых входных байт: ASCII, control, CR/LF и high-bit значения. Модель независимо выводится из source mask/print/count: 36 wire bytes и 384 stdout bytes.

Raw/final VM и actual assembled Z80 проходят ordinary/Grace Size/Grace Speed. Native модуль 253 байта, main 109087 T во всех режимах; Grace выигрыша нет. Root exported повтор PASS 0,070 с. Весь main и net_send исполняются, не заменяются моделью. Native test-owned linker добавляет отдельные RET к четырём declared network/control hooks; actual net_write ABI A и net_read result A. Неожиданный control call/input/port/foreign PC запрещён; возврат PC7003/SPF000; полный RAM сохраняется кроме фактического стека. Print port23 проверяет low/high address bytes.

Это явная тестовая сеть, не public IRC server или TCP bridge, и один конечный сценарий. Статус independent bounded VM/native относится к этой нагрузке. Исходники, frozen wire/stdout, unlinked/linked ASM/BIN, fullRAM и MIR сохранены.
