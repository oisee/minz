# Original map/filter/forEach: ordered native evidence

Два entry исходного `examples/nanz/03_filter_map_chain.nanz` — `filter_map_forEach` и `filter_map_explicit` — независимо проверены на raw MIR VM и фактически собранном Z80. Исходник не изменён. Компилятор и backend не менялись; miscompile в этой матрице не обнаружен.

Спецификация: [Language Book v7, глава 5](../Nanz_Language_Book_v7.md#chapter-5-iterator-chains). §5.1: `ptr.map(λ)` — «Transform each element», `ptr.filter(λ)` — «Keep elements where λ is true». §5.2 показывает `LD A,(HL); INC HL; ADD A,A; CP 11; ... CALL process`: чтение последовательных адресов, затем map, filter и callback. Это определяет независимый oracle; doubled byte вычисляется modulo 256 до unsigned сравнения. Пустой буфер не вызывает callback.

В исходнике нет main, globals или asserts. Его объявленный внешний `process(u8)` получает явную реализацию runtime, которая пишет каждый принятый байт в trace и увеличивает 16-битный count. Native `poke` имеет проверенный ABI HL/C. Этот адаптер не заменяет и не сокращает ни одну из исходных реализаций алгоритма.

Матрица: два entry × длины 0,1,2,127,128,255 × пороги 0,1,126,127,128,254,255 = 84 запуска. Буфер — перестановка значений байта с переполнениями при удвоении. Независимый Go model проверяет точный чередующийся порядок чтений input и записей survivor callback; все 260 байтов input, все 258 байтов trace вместе с неизменённым хвостом и guards, count, void-result. Native runner проверяет границы стека, PC/SP возврата, допустимые instruction extents и запрет доступа к code как к данным. Это workload без OS и без утверждения о физическом железе.

Результат: **PASS**, 1 408 236 T-states за все 84 вызова; исполняемый образ с runtime — **99 байтов**, SHA256 `93b4298eedb9b065e5f1eacceac0cd692a12f156d2e08f711b59da3f3ae2b616`. Raw/native полный capture и независимый model сохранены в JSON.gz, также сохранены workload, точный исходник, runtime, MIR, ASM и BIN.

Каталог сохраняет все 193 exact paths; теперь 24 имеют явно ограниченное независимое native доказательство. Новые 42 запуска второго entry — не ещё одна программа. Остальные пути не объявляются проверенными по одному compile/assemble PASS. Focused проверки включают существующие range-stage/capture regressions; подробные логи сохранены. Полный медленный pipeline не запускался.

Повтор: `GOCACHE=/tmp/minz-grace-gocache go test ./pkg/pipeline -run '^TestOriginalFilterMapOrderedTraceVMZ80$' -count=1 -v` из `minzc`. Экспорт: `MINZ_FILTER_MAP_ORACLE_ARTIFACTS=/absolute/path`.
