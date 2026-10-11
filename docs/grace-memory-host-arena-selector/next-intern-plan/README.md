# Следующий bounded kernel: append-copy в intern_region

Это read-only план, не начатая оптимизация. До закрытия текущего selector этапа
production и прототипы нового kernel не меняются.

Сохранённый PC-профиль исходного fed98 baseline (1221494 T) показывает:
`intern_region`334605 T, но из них254254 T находятся в copy-body диапазоне
0x06DC..0x0760 (1756..1888). Это75.99% intern и20.82% всей программы.
Scan setup/head+scan body/comparison вместе54481 T. Следовательно короткие
сравнения не главный источник27.39%; следующий приоритет —286 реальных
append-copy итераций с дорогими global word loads, u8 spills и pair shuffles.
`hotspot-breakdown.json` привязан к SHA исходного PC-файла и точным диапазонам.
Это exclusive native PC costs, не оценка возможного ускорения.

Минимальный source-bound candidate: exact append-loop
`poke(str_bump+1+j, peek(src_buf+start+j)); j=u8(j+1)`
заменить на два carried u16 pointer и тот же u8 count. Header poke(len), scan,
new_p и финальный str_bump update остаются исходными. Порядок на каждой итерации
обязателен: прочитать source, затем записать destination; следующий source-read
происходит после предыдущей записи. Alias/overlap должны сохранить последовательное
поведение. Нельзя заменять на memcpy, widened load/store или читать наперёд.

Первый proof требует closed peek/poke byte descriptors, полную исходную функцию/
модуль и exact fresh origins, отсутствие сторонних policy/lineage, exact pointer
u16 wrapping и u8 len0/1/255 semantics. Loop condition с globalstr_bump/src_buf
и существующими hosts допускает pointer hoist только при source expression proof:
peek/poke не меняют globals, body глобальных присваиваний не содержит. Обычные
region accesses способны численно попасть в адреса globals, но текущий закрытый
native contract запрещает unproved global accesses; MVP должен явно проверять
тот же source-global exclusion и не превращать численную адресную проверку в
общую purity-гипотезу. Переполнение адреса должно происходить на тех же выражениях
и давать тот же denied-first/second prefix.

Предпочтительно два source-bound варианта для измерения:
1. hoist только `src_buf+start` и `str_bump+1`, индекс j оставить;
2. полностью carried pointers, j оставить только как u8 trip counter.
Измерять сначала каждый отдельно, затем допустимое сочетание с first-byte peel
через отдельный compound proof; не приписывать исходные loop counts хвостам или
переписанным циклам. Не расширять существующий selector произвольнымfactory.

Условие валидации: raw independent tables len0/1/2/255, every mismatch неприменим
дляcopy но every failed-source/read or failed-destination/store position важен;
точный denied prefix, equal-value writes, src==dst, overlaps ±1, addresswrap,
readonlydestination, header partialfailure, persistentarena andresetstates.
Затем actual native boundary calls+полный30 SelfLanz, source/profile association,
allglobals/regions/full orderedevents and nativebyte/op budgets. Potential frame/
spill/codegen failures надо чинить отдельно без source weakening.

Выбор speed/size по двум реально измеренным target baselines; tieoriginal.
Никакого заранее обещанного процента выигрышa. При негативной стоимости сохранять
original и evidence. После стабилизации — independently reviewed exactprecompile
proof + complete source-bound typedprofile replay, immutablefactory retention,
fullgates/16CLI/fourbudget masks once. Target instruction counters, source trip
counters и semantic legality остаются разными доказательствами.
