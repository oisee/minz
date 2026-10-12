# Уточнение NC native диагностики

Утверждение304 о зацикливании как backend miscompile было преждевременным. Стенд размещал globals внутри ROM-защищённой области ZX эмулятора. Native требует flatRAM; после явного SetROMEnd(0) программа достигает tail-call reset. Стенд теперь принимает только точный исходный tailcall tui_reset с loaderreturnPC C003/SP EFFE, и main возвращается к C003/SP F000. Никакого source или compiler fix для этих двух ошибок стенда нет. Старые логи не переписаны.

После обеих поправок остаётся **настоящее расхождение native вывода** во всех3режимах: event191 выдаёт пробел вместо первой буквы H имени HELLO.TXT. Frozen source/disk/terminal oracle сохранены. Raw/finalVM совпадают с oracle. CorpusNC всё ещё pendingnative, настоящее происхождение нового mismatch пока расследуется. HIR local-array pointer truncation ранее независимо подтверждён и исправлен; новая harnessкоррекция его не отменяет.
