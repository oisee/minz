# Z-machine V3: подготовка Spectrum 128K

`zvm_core.nanz` — промежуточная версия интерпретатора с единым интерфейсом
памяти. Она пока использует host I/O и массив story на 131072 байта и **не
является программой для Spectrum**. Исходный VM showcase сохранён отдельно.

Все индексные обращения story из логики интерпретатора перенесены в
`memory_read`/`memory_read16`/`memory_write`. Загрузка, restart, save и restore проходят через
`memory_load`/`memory_save`/`memory_restore`; native-версия должна заменить
эти реализации и размещение данных. Текущий snapshot ещё не сокращён.

Воспроизвести промежуточный исходник:

```sh
python3 scripts/zmachine/prepare_memory_core.py
```

Проверить существующие независимые VM-фикстуры, включая код выше 64 КБ,
потоки, Unicode, формат save и отказ restore чужой версии story:

```sh
ZVM_CORE="$PWD/examples/zmachine/spectrum128/zvm_core.nanz" \
  bash examples/zmachine/z3-fixtures/smoke.sh /path/to/mzv
```

Сравнить игровой transcript и байты сохранения исходной и новой VM-версий
на отдельно предоставленной story:

```sh
python3 scripts/zmachine/check_memory_core.py /path/to/mzv /path/to/story.z3
```

Это дифференциальная проверка двух интерпретаторов, не независимый оракул
правил игры и не доказательство исполнения Z80. Для точного MIT Zork I
сохранён исходный gate с SHA256 и сравнением Frotz:

```sh
ZVM_CORE="$PWD/examples/zmachine/spectrum128/zvm_core.nanz" \
  bash examples/zmachine/zork-west-demo/full-zork-smoke.sh \
  /path/to/mzv /path/to/pinned-mit-zork1.z3 /path/to/dfrotz
```

Банковый читатель в `stdlib/zx/story_memory.nanz` проверен отдельно на Z80,
но ещё не подключён сюда. Следующий шаг — native размещение рабочих данных
и backend памяти с банковым доступом, затем вывод и ввод Spectrum.

Измерение рабочих кадров на выбранном transcript:

```sh
python3 scripts/zmachine/profile_state.py /path/to/mzv /path/to/story.z3
```

Первый маршрут дал 11 кадров и 11 слов стека выражений; это **не предел всей
игры**. Экспериментальная конфигурация с 32 кадрами и 256 словами сохраняет
проверки переполнения и расходует 1728 байт на locals, stack и return metadata:

```sh
python3 scripts/zmachine/compact_state_core.py /tmp/zvm-compact.nanz
python3 scripts/zmachine/check_memory_core.py /path/to/mzv /path/to/story.z3 \
  --core /tmp/zvm-compact.nanz --different-save-format
```

У неё собственная длина save: сверяется игровой вывод и успешное продолжение
после restore, а равенство байтов с прежним форматом не заявляется. Прежний
формат основной VM-версии сохранён. На существующих фикстурах компактный save
имеет SHA256 `df95a1d269ebc6f0163adc944f71c7a95021df71948a375d53a5e7e81415dd13`:

```sh
ZVM_CORE=/tmp/zvm-compact.nanz \
ZVM_EXPECTED_SAVE_SHA=df95a1d269ebc6f0163adc944f71c7a95021df71948a375d53a5e7e81415dd13 \
  bash examples/zmachine/z3-fixtures/smoke.sh /path/to/mzv
```

`compact_state_core.py --size-diagnostic` создаёт **неисполняемый** размерный
пробник с маленькой story и host-заглушками. Его нельзя использовать как игру.


Горячие обращения с 16-битным адресом используют `memory_read16` и узкий
`memory_write`. Чтение инструкций и checksum сохраняет `u32` адреса.
Банковая библиотека дополнительно предоставляет `story_byte_split` и
`story_word_split`: младшее слово адреса и номер 64-КБ части передаются
раздельно. Перенос при чтении слова на 65535/65536 обрабатывается явно.

На одной матрице из 220 байтовых/словных чтений оба ABI проходят Z80:
широкий — 846816 T, раздельный — 237116 T. Это сумма конкретных вызовов
в Pentagon-модели без contention, не скорость всей игры. Размер модулей
с обёртками — 1022 и 825 байт; в обоих есть импортированные примитивы.

При одинаковой компактной конфигурации и backend узкий интерфейс уменьшил
область функций неисполняемого диагностического пробника с 27414 до
23439 байт. Globals остаются 4169 байт; полный пробник — 27608 байт.
Фикстуры VM и исходный формат save проходят без изменений.

Первый native harness (`prepare_native_core.py`) исполняет настоящее ядро
с банковой story и захватом вывода в RAM. Ввод, save и restart Spectrum ещё
не реализованы. Независимая Inform-фикстура stream3 находится в банке 0.
Код размещается от $5B00, native stack ниже $C000, IRQ отключены.

**Интеграционный gate проходит:** ядро завершает 41 инструкцию stream3,
вывод и байты трёх вложенных memory streams совпадают с независимым эталоном.
Это проверка одной фикстуры; полный Zork и Spectrum-консоль ещё впереди.
Объявления globals декодера должны предшествовать его коду: иначе парсер не
видит u16-типы и сужает адресные выражения. Воспроизведение:

```sh
cd minzc
go test ./pkg/pipeline -run '^TestZMachineBankedInterpreterNative$' -count=1 -v
```
