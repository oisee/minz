# Корпус: production CP/M и pure rotozoomer

Каталог сохраняет 194 точных пути. `51_impl_cpm_demo.nanz` теперь имеет native CP/M runtime результат: original main через public CLI COM, три режима и два BDOS2 host контракта, OS RET/PC/SP/fullRAM/guard, точные geometry/decimal ожидания. Его pure методы также имеют отдельный raw/final VM proof. VM ASM/BDOS output не симулируется, hardware не проверялось.

В результате inline-ASM-only путей стало 31, native CP/M runtime — два. 133 independent/bounded VM/native и два native SMC сохранены. Остальные категории и их ограничения не переименованы ради PASS.

Rotozoomer получает положительные exact-source доказательства всех четырёх чистых функций, но остаётся external-runtime-needs-audit для rendering/main/canvas. Полный native renderer этим не обещается. Source/test hashes и root runtime/pure logs проверяются registry; focused catalog PASS 2,895 с.

Свежий полный CLI начат после CP/M production и отрицательных литералов. База кода2a3f6662; текущая незакоммиченная правка касается только пути metadata каталога216→224. Session31257, terminal результат ожидается. Долгий pipeline29df08bb остаётся прежним executable и не перезапускался.
