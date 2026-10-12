# MZX SQLite native protocol compatibility

Base `7c4ce479`. Freeze `b96fdd7d` предшествует выполнению; `31a7a008` сохраняет настоящий отрицательный результат. Проверка вызывает штатный `setupMZXSQLitePorts` на `spectrum.New48K` и публичные `Ports.ReadPort/WritePort`, с реальным modernc SQLite, без копирования provider logic в oracle.

До исправления: open через IN43 возвращает **0 вместо handle1**; ошибка query возвращает **1 вместо handle0**. Native stdlib читает именно status43, а MZX хранил handles/step в resultLo на45 и ставил status0. Поэтому ошибку нельзя объяснить compiler ABI. SQLite cmd MZE и stdlib source snapshots сохранены как authority; MZE не изменён.

Fix `f6e6efe6` меняет только MZX provider: status43 получает handle open/query и 1/0 результата step; open/query errors дают0; command handler очищает strBuf после **любой** команды, включая early errors и handle-only step/finalize/close. Port45/47 data values сохранены. Это устраняет попадание handle bytes в следующую SQL строку или путь. Тест нового SELECT256 после двух строк/finalize, а также close/reopen handle2, проверяет lifecycle на настоящей базе.

Публичные protocol tests: Alice/-32768, Bob/65535, step row/done, invalid query/exec, invalid stmt/column, integer values 0,1,255,256,32767,32768,65535,-1,-32768 по модулю65536 и строки0/1/17/255+NUL. Это bounded набор; исчерпывающий SQL dialect не заявляется. Handle namespace остаётся прежним byte namespace (поведение после255 handles не изменялось/не доказано). sql.Open сохраняет прежний lazy connection/error lifecycle; недоступный файл не превращён в отдельный новый contract.

Дополнительно неизменённый original sql_test6 компилируется ordinary/GraceSize/GraceSpeed и выполняется на Z80 с **реальным MZX provider**, явно test-owned BDOS2 полным clobber A/F/BC/DE/HL/IX/IY. Каждый вариант 1670B; точные59 stdoutbytes `Open...\nExec...\nQuery...\nStep1...\nAlice\nStep2...\nBob\nDone.\n`, 11команд, status sequence1,0,0,0,1,1,1,0,0. Проверяются returnPC/SP и все65536RAMbytes: разрешены лишь actualstack и expected Bob+NUL+e+NUL recv buffer. Более подробные ordered byte observations/ASM/BIN/whole-memory snapshots уже сохранены в предыдущем `reports/sqlite-whole-main` packet; этот новый тест связывает тот native consumer именно со штатным MZX provider. Он не содержит заново выдуманной таблицы rows.

Все headless `cmd/mzx` tests PASS0.185s. GUI/GLFW build/runtime и полноценная Spectrum machine framebuffer не исполнялись. Загрузка приложения test-owned ordinary image, а не новый productionCPM link или hardware claim. Provider fix не затрагивает RuntimeCPM, HIR, stdlib, allocator или MZE. `_sql_recv_str` по-прежнему требует NUL ≤256bytes, это не исправление произвольного overflow.

Репродукция из minzc:

```sh
go test -tags mzx_headless ./cmd/mzx -v
```

Frozen executable `mzx.test` находится на шаре, SHA в build.json. Его cwd должен быть `minzc/cmd/mzx`, чтобы retained original source paths находились правильно. `frozen-test.txt` — terminal результат именно этого executable, а `before.txt` — counterexample до production fix. `production.patch` фиксирует точную минимальную production разницу.

Canonical share: `/mnt/safe/minz/grace/2026-10-12/mzx-sqlite-protocol/`.
