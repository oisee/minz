# Интеграция 087: FAT12 live-A correction и полный source admission

Backend a043fa78 исправляет implicit A scratch при byte load из BC/DE в spill. Раньше этот путь обходил сохранение живого AF; HL spill сохранял AF отдельно. Теперь BC/DE сохраняет AF ровно один раз, HL путь не менялся. В FAT12 последний байт имени 84 попадал в pdrv аргумент disk_write вместо0.

125 прямых ISA случаев и18 actual FAT12 slot/mode случаев прошли. Native disk_write получает ожидаемые drive/sector/count/buffer, raw/final VM и native512bytewindow/all64KiB/SP проходят. Root focused с originalidiomatic sum PASS12,883s. Полный MIR2 агента PASS8,299s; новый root core5fe165dd уже прошёл HIR25,399s/MIR210,982s/Nanz14,625s/VIR2,111s, CLI ещё выполняется. Полный pipeline3e143f9f session71745 жив и предшествует исправлению; новый package PASS не заявляется.

Memory-host d99df8ad допускает unbound extern только если он недостижим из полного исходного direct-call graph и candidate graph. Retainedextern callable identity проверяется целиком; функция/address-taking/indirect escape глобально запрещены. Defensive VM hooks и native unreachable-fetch отказ сохраняются. Source-reachable unbound call нельзя скрыть оптимизацией. Namespace prefix не даёт доверия privatehook.

Оба iterator sum теперь проходят все1024calls с полным исходником плюс observer, без удаления deadextern. Machine BIN/T unchanged. Прежний linker-adapter packet оставлен историческим, новый sourceprovenance сохранён отдельно. Root admission/native tests PASS. Эта проверка не доказывает все194showcase.

Оба новых пакета и standalone reports copiedbyteverified наdatedsharecompiler-docs; SHAmanifest каждогоpayload проверен. Адресной hoist в correctnessкоммитах нет: он продолжает измеряться наисправленномbaseline отдельно. Общая цель активна.
