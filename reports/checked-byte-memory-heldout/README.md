# Перенос memory shape-фрагментов на другие спрайты

Сохранённый fragment pool прежнего шестибайтового linear masked sprite
используется без новых назначений для трёх новых descriptors. Меняются
маски, constants, адреса, число операций и имя функции (`heldout_draw`).
Для каждого исходника полный MIR/ABI заново допускается через
`ComposeCheckedLocalTables`; исторический ключ не разрешает emission.
Обычный Size-selector выбирает меньший BIN, а native timing проверяется
после выбора и не используется для подготовки фрагментов.

| Descriptor | Байты ordinary / selected | Такты ordinary / selected |
|---|---:|---:|
| ZX: x=29, y=191, ширина 3, PAPER bitmap boundary | 26 / 25 | 114 / 110 |
| Linear: другая база, stride 12, x=2,y=1, ширина 6 | 46 / 43 | 198 / 186 |
| Linear: stride 9, x=1,y=1, ширина 4 | 36 / 34 | 156 / 148 |

Все результаты одинаковы в greedy/PBQP контрактов × HIR+MIR Grace off/on,
OptSize=true. Размер первой строки — три байта bitmap, не три пикселя;
для ZX используется независимая адресная перестановка из Antique Toy,
глава 2. Это выполнение CPU-кода без Spectrum ROM/ULA contention.

Каждый baseline и selected запускается на 256 полных фонах RAM и initial
flags: 3 descriptors × 4 compiler modes × 2 executables × 256 = 6144 native
случая. Оракул сверяет все 65536 байт, код, стек, permitted picture bytes и
return PC/SP. Raw и optimized MIR VM проверены по тому же независимому
mask oracle для всех фонов. Исходный pool перечитывается после каждого
подтеста и побайтно остаётся прежним.

PASS за 2,589 с. Тест включён в fast inventory; полный релизный корпус не
повторялся. Это перенос локальных назначений на другую форму и constants;
дополнительного вклада PFCCO/Grace здесь нет. Dynamic base и обычный
measured CLI пока требуют отдельного стыка: controlled workload наблюдает
именованные globals/resources, а эти descriptors используют fixed addresses.

Результаты не расширяют контракт admission: byte LOAD/STORE в leaf block,
канонические 16-битные pointers и отсутствие CALL остаются ограничениями.
Большие спрайты с более чем восемью отдельными фрагментами не доказаны.

Все исходники, pools, exact ASM/BIN, таблицы и native-логи сохранены на шаре
`/mnt/safe/minz/grace/2026-10-14/checked-byte-memory-heldout/manifest.json`.
В этом отчёте сохранены metrics и полные свидетельства каждого режима.
