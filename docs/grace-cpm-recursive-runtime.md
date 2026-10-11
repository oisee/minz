# Исходный selfLanz: COM с ограниченным рекурсивным стеком

Обычная CLI собирает исходный `examples/nanz/self_lanz_parser.nanz` без изменения алгоритма или текстовых исправлений ASM:

```sh
mz examples/nanz/self_lanz_parser.nanz -t cpm --runtime cpm --runtime-stack 1024 --asserts none -o parser.com
mz examples/nanz/self_lanz_parser.nanz -t cpm --runtime cpm --runtime-stack 1024 --asserts none -o parser.a80
```

`--runtime-stack` — явный выбор guarded application stack: чётное число 64..16384, с дополнительной проверкой полного образа ниже 8000. Без него остаётся прежний 256-byte runtime с проверкой статической границы и отказом для рекурсивных CALL. Без `--runtime cpm` параметр стека отклоняется до записи выходного файла. Нулевой параметр отсутствует в JSON options, прежние profile IDs сохранены. Измеренные profile APIs application runtime не принимают.

Каждый генерируемый PUSH/CALL предваряется guard. Его успешный путь сохраняет AF, HL и остальные регистры, включая результат и flags. Проверка сама использует шесть байт (CALL return word + AF + HL); успешный guard оставляет ещё восемь байт запаса. Поэтому следующий рост на два байта и следующий guard не пересекают stack guard. Console wrapper использует ту же защиту перед собственными PUSH и BDOS CALL. При переполнении статус `__cpm_status=1`, стек возвращается к сохранённому incoming SP и выполняется RET к вызывающей CP/M; normal entry устанавливает статус 0. Внутренние потребности BDOS в стеке остаются условием ОС, не входят в доказанную application границу.

Статическая проверка требует guard около каждого роста и точного тела checker; проверяет баланс всех локальных путей функции, включая условные RET и собственные сохранённые слова EX(SP). Рекурсивные CALL допускаются при guard, глубина ограничивается runtime. Guarded режим отключает ASM peephole: старый проход может вводить новые CALL или нарушать соседство проверки с ростом стека. Обычный port23 и статический CP/M режим не меняются. Это первая корректная граница рекурсии, не заявление об оптимальном или быстром runtime.

Абсолютные spill homes сохраняются нативным caller-save до копирования аргументов и восстанавливаются после возврата. Отдельный native regression принудительно помещает параметр, живой после настоящего recursive CALL, в один и тот же именованный spill slot всех активаций. Для n=0,1,2,7,12,31 результат совпадает с независимой рекуррентной формулой по модулю 65536 и VM; реальные CALL и общий slot проверяются в emitted ASM. Это конкретная проверка reentrancy, а не доказательство всех возможных типов/рекурсивных программ.

Frozen CLI COM: 4727 байт, SHA256 `4446673251c6b756d50e36f236c33c1c7af2270170ed322b34f78f59046b5097`. Native запуск неизменённого main: 983310 T, 135 байт stdout, 17 узлов; наблюдаемый resident stack затронул 50 байт. Независимый host parser сверил дерево и интернирование, все globals и полные node/string/source arenas; все 65536 байт проверены против исходного состояния с разрешением изменений только в этих аренах, именованных globals/spills, status/saved-SP и stack. Код и literals сохранены. BDOS hook портил все первичные регистры/flags; PC=0 и incoming SP восстановлены.

64-byte fixture с настоящей глубокой рекурсией даёт controlled overflow: status1, result не опубликован, guard сохранён. Проверены пороги guard и сохранение registers/flags; malformed sizes отклоняются до записи. Отрицательная admission проверяет отсутствующий guard, изменённый checker и несбалансированный local frame. Полный MIR2 PASS 13.898s. Ранее сохранённые оригинальные default/translated main tests и tokenizer production runtime остаются PASS; прежние результаты не переименованы.

Это машинный COM с CP/M ABI, запущенный в Z80 эмуляторе с BDOS2 hook; настоящая CP/M OS пока не проверена. Исходник использует fixed arenas 8000..afff и c000..cfff, требует TPA до d000 и доступного BDOS. Runtime не превращает произвольный `poke` в sandbox, пользовательские arenas и input bounds сохраняют исходный контракт. Практическая оптимизация стоимости guards и запуск на реальной CP/M — дальнейшие шаги.

[Исходник, ASM/COM, oracle, overflow capture и логи](grace-cpm-recursive-runtime/artifacts/). Frozen compiler `mz-cpm-recursive-final`, SHA256 `384b644a8fdf39446e2ccccff71497a267280a8e40bdad29b4a7852387730ad0`; включает backend fix ee5621ac (локальная копия bec7b4dc).
