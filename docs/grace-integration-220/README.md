# Свежий core и расширенные композиционные проверки

Полный core на checkpoint `166103ff` после C89 statement/lexical/expression и Z80 predicate-truncation исправлений завершился exit0: HIR PASS 27,483 с, MIR2 PASS 72,771 с, Nanz PASS 14,490 с, VIR PASS 2,365 с. Тесты исполнены одним исходным go test запуском; логи сохранены без подмены прежних checkpoints.

Root повтор unchanged a1/a2/a4/a8: PASS 14,989 с, 10 616 832 native CALL плюс столько же raw/final VM проверок. Все пары byte a/b и заранее фиксированные границы c/d, три режима. PC/SP, память и реальные moves включены в проверку и costs; полный 256³/256⁴ Cartesian не заявляется. Все 70 агентских payloads SHA проверены в Git/шаре. Точные root ASM/BIN/MIR/options/results рядом.

Production CP/M расширение ещё в работе; эта core проверка ему предшествует. Retained полный pipeline `29df08bb` подтверждён живым процессом, terminal результат остаётся pending; его не перезапускали из-за продолжительности наблюдения.
