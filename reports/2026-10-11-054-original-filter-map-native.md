# Оба исходных map/filter/callback entry проверены на Z80

В 81df76ed добавлена независимая проверка точного examples/nanz/03_filter_map_chain.nanz, без изменения исходника или backend. Оба полных entry filter_map_forEach и filter_map_explicit исполняются с явным runtime adapter process(u8), который сохраняет ordered trace.

84 вызова охватывают длины 0/1/2/127/128/255 и thresholds 0/1/126/127/128/254/255. Независимая модель следует Language Book v7 §5.1/5.2 и unsigned byte wrap: сверяются точные ordered reads/writes, весь input260, trace258/guards/count, результаты. Стенд проверяет PC/SP и неизменность кода. Raw VM и assembled Z80 проходят. Это проверка двух вызываемых функций с явным runtime, не whole-main или физическая ОС.

Карта сохраняет все 193 исходника; bounded behavioral coverage теперь 24, сборка корпуса отдельно 192 PASS/1 отказ ZVM. Все 16 payloads исходного пакета SHA256 проверены и скопированы на /mnt/safe/minz/grace/2026-10-11/compiler-docs/. Интегрированный native тест также прошёл, лог сохранён отдельно. Большой pipeline-cc502-full продолжает выполняться.
