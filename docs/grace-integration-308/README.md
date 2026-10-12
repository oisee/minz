# Z80 addr_of: сохранение полного spill-адреса

Fix `36caa041`: OpAddrOf spill теперь пишет полный16-bit word и сохраняет живой HL через PUSH/POP. Прежний codegen намеренно записывал лишь lowbyte и портил HL. Independent native regression доfix FAIL: target8013 сохранён0013, liveHL потерян, outputнеwritten. Послеfix совместно соtypedwordstore cases **PASS0,053с**, fullRAM/targetslot/liveHL/PCSP проверены.

Послеfix nativeNC выдаёт правильные имена и размеры, но остаётся отдельный mismatch event434: read-only flag печатаетсяR вместо'-'. Во всех3режимах wholemainFAIL; NCstatusнеповышен. Исходник/образ/expectedeventsнеизменны. Следующий уровень расследования—живойattr черезвызовыdraw_info_panel.

FullrootMIR4e70ff9e послеPFCCO интеграции **PASS81,158с**; предшествуетновомуaddr_of fix. НовыйfullMIR36caa041 работаетотдельно. Полноеудовлетворениецели пока не доказано.
