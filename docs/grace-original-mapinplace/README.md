# Original mapInPlace: cumulative native array behavior

Exact source: `reports/showcase-src/2026-03-10/ex7_mapinplace.nanz`. Его единственный entry `add2_inplace(^u8,n)` изменяет массив на месте; исходник не менялся. Main, asserts, imports и extern отсутствуют. Проверка исполняет весь исходный entry; компилятор/backend не менялись и miscompile не найден.

Спецификация: [Language Book v7 §11.3](../Nanz_Language_Book_v7.md#113-mapinplace-djnz-direct-prealloccoalesce), «mapInPlace: DJNZ Direct», пример `add2_inplace` с преобразованием `x + 2`; [§2.6](../Nanz_Language_Book_v7.md#26-pointers-and-arrays) задаёт байтовые указатели/массивы. Независимый Go model вычисляет каждый результат modulo 256 и сохраняет память между вызовами.

Основная матрица: 20 persistent вызовов исходного entry с длинами 0,1,2,127,128,255, повторными и перекрывающимися участками. Данные включают все 256 byte values и переполнение 254/255. Проверяются все 1030 байтов трёх областей, включая guards и неизменённые хвосты, точный чередующийся порядок read→write, void-result и отсутствие globals. Raw MIR VM и собранный native Z80 сравниваются с моделью после каждого вызова. Native runner контролирует PC/SP возврата, границы стека и допустимые обращения к instruction/data storage.

API bounded memory-host требует integer entry arguments и зарегистрированный host. Поэтому явный test-only driver преобразует `u16` в `^u8` и вызывает исходный `add2_inplace`. Отдельный read-only observer нужен для host-контракта; его один дополнительный вызов не считается вызовом алгоритма или новой программой. Сохранены точный source, полный driver, workload, ASM/BIN/MIR, raw/native captures и независимый model.

Основной результат: **PASS**, **118 329 T / 22 B**, SHA256 `3644107cb1297e339a418ac0a545d5a9aa7392870115b80fad9e57df2ea453ee` за 20 algorithm calls плюс observer.

Отдельно проверен настоящий адресный переход `0xffff → 0x0000`: девять cumulative вызовов того же entry через constant-address drivers, raw ordered byte events, native полные 516 байтов двух областей вместе с guards, PC/SP, границы стека и неизменность образа. Wrap image **92 B**, SHA256 `4a0b9b1458eb0d205434e6daeaac0f98a6999bb23352ca65f1b06942975159b7`. На этой матрице native byte order отдельно не инструментирован; полная последовательность памяти сверяется после каждого вызова.

Платформа wrap — явно flat all-RAM Z80. Bounded memory-host запрещает область по адресу zero из-за VM function-tag storage, поэтому этот admission не ослаблялся. Direct-entry VM не использует function tags как указатели. Native emulator настроен `SetROMEnd(0)` и `SetSMCTracker(nil)`: его default Spectrum ROM блокирует записи в low RAM, а screen SMC hook ошибочно трактует старый byte данных с кодом RST как вызов экрана и меняет A. Сохранённые `initial.txt`/`raw-error.txt` и `wrap-before.txt` отражают ограничения выбранного harness до явной настройки платформы, а не баг компилятора. Не утверждается возможность писать Spectrum ROM или запуск на физическом железе.

Каталог сохраняет все 193 exact paths: теперь 25 имеют ограниченное независимое native доказательство. Дополнительный wrap режим не увеличивает число программ. Новый pair плюс existing filter-map oracle PASS; полный медленный pipeline не запускался. Все payloads в `manifest.json` имеют SHA256.

Повтор из `minzc`: `GOCACHE=/tmp/minz-grace-gocache go test ./pkg/pipeline -run '^TestOriginalMapInPlace' -count=1 -v`. Экспорт: `MINZ_MAPINPLACE_ORACLE_ARTIFACTS=/absolute/path`.
