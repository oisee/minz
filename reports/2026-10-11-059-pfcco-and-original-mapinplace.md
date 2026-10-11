# PFCCO cost repair и исходный mapInPlace

Интегрирован b12b2d49: PFCCO PBQP учитывает call demands на рёбрах и non-call body demands в unary; legal byte transfers A/B/C стоят 4 T вместо ложного InfCost. Greedy не изменён. Независимые native fixtures применяют контракты до PBQPAllocate и включают реальные calls/moves/saves: offset=2*x+3 прямой 47 360→46 336 T, 25→24 B; условный 52 645→49 086 T, 30→29 B. Исходный 3+x сохраняет нерешённую стоимость call-save pressure, отрицательный контроль не удалён. После интеграции все 3 новые MIR2 группы PASS 1,378 с. Оптимальность произвольных графов и обычный pipeline opt-in этим коммитом не заявляются; opt-in реализуется отдельно.

Интегрирован 0be9ab59: точный archived ex7_mapinplace.nanz без source/backend изменений. 20 cumulative calls+observer проверяют 1030 байт RAM и ordered events; 9 отдельных wrap calls проверяют 516 байт RAM, raw VM порядок и native SP/code. FlatRAM стенд явно отключает ZX ROM и SMC screen hook; найденные ложные падения стенда сохранены и не названы miscompile. Две новые группы после интеграции PASS 0,064 с. Карта сохраняет 193 paths, 25 имеют bounded independent evidence.

Пакеты и интегрированные логи проверены SHA256 и сохранены на /mnt/safe/minz/grace/2026-10-11/ в compiler-docs и compiler-reports соответственно. Большой pipeline остаётся live, полный текущий PASS не заявляется.
