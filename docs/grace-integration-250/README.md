# Полный core после Pascal decimal и coalesce

Checkpoint `81649477`: единый запуск HIR/MIR2/Nanz/VIR/Pascal завершился exit 0. **HIR PASS 29,637 с; MIR2 PASS 75,080 с; Nanz PASS 22,515 с; VIR PASS 2,337 с; Pascal PASS 0,029 с.** Код включает signed/unsigned decimal formatter, native CP/M adapter/private label checks, safe group coalesce и ZX source ABI/string fixes.

Пять heldout и showcase compile/assemble на том же compiler checkpoint уже PASS в пакете249. Полный CLI запущен отдельно с заранее сохранённым `cli-81649477.test`, SHA256 `97699ec17b58402b3d598ee2f4aa37843504dfd8ba454c751eed6a17b73e6cd6`, cwd `minzc/cmd/minzc`, `-test.count=1 -test.timeout=30m`; terminal ещё неизвестен. Название executable относится к неизменному compiler checkpoint; перед сборкой изменился только документационный commit ad961834.

Retained pipeline29df08bb всё ещё исполняется отдельно на более старом executable. Current core PASS не подменяет его результат и не доказывает весь native/runtime корпус.
