# RST dispatch belongs to execution, never memory writes

Исправлена production ошибка `NewRemogattoZ80WithScreen`: изменение обычных данных со старым байтом `C7/CF/D7/DF/E7/EF/F7/FF` запускало RST hook через SMC tracker. Независимый Z80 `LD (HL),A; RET` воспроизвёл ошибку для всех восьми значений; `DF/E7` дополнительно меняли посторонний RAM `ffff`. `SetRegisterA` писал туда результат keyboard hook вместо изменения A.

Исправление удаляет SMC→RST связь. RST hooks вызываются только при реальном fetch unprefixed RST opcode в общем `safeDoOpcode`, который используют Step и Run (включая вызовы через embedded base emulator в profiling). `SetRegisterA` меняет actual CPU A и не пишет память. Пользовательский SMC observer остаётся обычным уведомлением о записи.

Handled hook сохраняет существующий контракт legacy `Z80WithScreen`: это атомарная host-ROM abstraction за 11 T, PC+1, SP/flags unchanged, refresh register increment; DF/E7 возвращают byte в A. Это не исполнение полного Spectrum ROM. Unhandled RST исполняется аппаратным CPU: 11 T, push PC+1 и переход к vector; последующий RET 10 T восстанавливает PC/SP. DD/FD-prefixed opcodes не декодируются hook слоем, остаются hardware path. Отдельный существующий Agon vector handler не менялся. Предположение host-hook API: callback меняет только эффекты, разрешённые его контрактом; это не универсальная ROM ABI.

Регрессии проверяют native stores для восьми старых RST bytes: все main registers/A/F, посторонний ffff, immutable code, SP и RET; исполнение всех восьми RST с handled hooks через Step/Run, ровно один callback и правильные A/PC/SP/flags/11T; refresh wrap; unhandled stack/vector+RET; patch RST→NOP→RST без раннего callback; DD/FD-prefixed path против plain CPU. Before/after логи сохранены. Полный emulator package PASS; профильный измеритель/refusals и обе прежние original-mapInPlace матрицы PASS 13.515 s. BIN/ASM исходной mapInPlace программы не изменились.

Изменения ограничены эмулятором и регрессиями. Старые captures/frozen hashes не переписаны: затронутые workload профили при необходимости требуют нового честного измерения. Не утверждается, что все исторические profiles были повреждены: воспроизведена конкретная native запись по старому RST-valued byte и выполненные RST hooks.

Повтор из `minzc`: `go test ./pkg/emulator -count=1`; focused `go test ./pkg/emulator -run '^TestScreen' -count=1 -v`. Артефакты отчёта перечислены в `manifest.json` с SHA256.
