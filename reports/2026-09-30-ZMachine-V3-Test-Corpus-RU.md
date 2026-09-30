# Малые Z3-истории и test suites — проверка 2026-09-30

Задача: найти проверяемый корпус для Nanz Z-machine v3 с перспективой 48K ZX Spectrum. Размер ниже относится к **story-файлу**, а не к готовому Spectrum-образу: интерпретатор, экран, стек и загрузчик требуют отдельного RAM budget. Все числа ниже измерены на локальных копиях указанных ревизий; исполнялись в `dfrotz` 2.44.

## Конкретные кандидаты

| История | Версия / размер | Лицензия и роль | Наблюдение |
|---|---:|---|---|
| [West of House micro-demo](../examples/zmachine/zork-west-demo/README.md) | Z3 / 2 378 байт | MIT Zork I derivative; smoke для запуска, текста и input | Исходник, двоичный файл и build script уже в MinZ; scripted transcript проходит. Это самодельное маленькое подмножество сцены, не исходный Zork parser. |
| [CZECH](https://github.com/jeffnyman/zifmia/tree/e1a088ec4a90d6299193d43a79095af269e72a92/testers/czech) v0.8 | Z3 build / 10 752 байта | Собственная разрешительная лицензия автора **не MIT**; conformance tests | Из `czech.inf` собирается `-v3`; в Frotz: 368 tests, 349 passed, 0 failed, 19 print tests для визуальной оценки. |
| [Dark Pit](https://github.com/akosela/darkzil/tree/d3ded977436cd96dc7f22371041b53e9cdb2f24e) | Z3 / 27 490 байт | MIT, готовая игра и исходник | Запуск/`LOOK` проверены в Frotz; включает комнаты, предметы, parser, инвентарь, свет и бой. Лучше как поздний integration workload. |
| [PunyInform `minimal.inf`](https://github.com/johanberntsson/PunyInform/blob/a855374a79b2ae807f2ef92777be02f9b4a2bf72/minimal.inf) | Z3 build / 25 600 байт | MIT-репозиторий с исходником и библиотекой | Самостоятельная сборка Inform 6.45; стартовый пример с parser/library. Лицензии и атрибуцию зависимостей перепроверить перед включением бинаря в релиз. |
| [PunyInform `cloak.inf`](https://github.com/johanberntsson/PunyInform/blob/a855374a79b2ae807f2ef92777be02f9b4a2bf72/cloak.inf) | Z3 build / 28 672 байта | MIT-репозиторий; адаптация Cloak of Darkness с отдельной атрибуцией | Компилируется и стартует в Frotz; полезная короткая история с parser/object puzzle после CZECH. |

Готовый `minizork.z3` в соседнем checkout занимает **52 216 байт** и потому не годится как *целиком резидентный* story для 48K. Полный воспроизводимый MIT Zork I — **86 928 байт**. Попавшиеся `czech.z5` (13 312 байт) и `etude.z5` (16 896 байт) не являются Z3-проверками: готовые файлы требуют V5.

## Подмножества CZECH

В [исходнике CZECH](https://github.com/jeffnyman/zifmia/blob/e1a088ec4a90d6299193d43a79095af269e72a92/testers/czech/czech.inf) функция `Main` вызывает отдельные группы: jumps, variables, arithmetic, logical, memory, subroutines, objects, indirect, misc, header и print. Аргумент `0` запускает группу, `1` пропускает. Автор прямо документирует этот механизм в README и исходнике. Большинство тестов используют явные Z-machine opcode, поэтому CZECH проверяет интерпретатор точнее, чем случайная игра.

Проверено: официальный Inform 6.45 (commit `d1066bc214a45ee0f600d2ae7f94ad0210606317`) собрал `czech.inf` командой `inform -v3 czech.inf czech.z3`. Z3-файл: **10 752 байта**, SHA-256 `6d628af7ab4779224cb87923cce3764e3ad3ab8634758f409b638ba20f91d283`. `dfrotz -m czech.z3` вывел `Performed 368 tests. Passed: 349, Failed: 0, Print tests: 19`. Это соответствует приложенному `czech.out3` по числу тестов и итогам; форматирование терминала и строки header зависят от интерпретатора.

Отдельно в копии исходника были пропущены все группы, кроме jumps и variables: `Performed 70 tests. Passed: 69, Failed: 0, Print tests: 1`. Файл **остался 10 752 байт** — переключатель уменьшает число выполняемых тестов, но не размер story. Для физически маленьких файлов по одному opcode нужен собственный генератор/набор ZIL fixtures, а не переименование режима CZECH в «малый бинарь». Эти fixtures можно проверить на Frotz и использовать в `mzv`/Z80 trace comparison.

Лицензия CZECH разрешает usage/distribution/modification при сохранении авторского текста лицензии и copyright notice, но она **не MIT**. Поэтому сейчас не копируем CZECH в репозиторий или релиз; на J1 храним pinned URL/SHA и процедуру локальной сборки. TerpEtude полезен позже для V5/сложного I/O, но не как gate первого V3-интерпретатора.

## Практическая лестница

```text
0. Z3 header + наш 2.4K micro-demo: старт, print/read, простые ветвления
1. Мелкие opcode fixtures с ожидаемыми PC/stack/memory, Frotz как oracle
2. CZECH v3: jumps/variables → arithmetic/logical/memory → calls/objects → full
3. Dark Pit v3: настоящая MIT-игра 27.5K с input и состоянием
4. 48K Spectrum: тот же корпус после измерения RAM map и загрузчика
5. Полный MIT Zork I: отдельный 128K/paging milestone
```

Переход между ступенями определять по совпадению состояния и вывода, а не по одному факту, что история открылась. CZECH главным образом покрывает вычисления и структуру VM; его README отдельно перечисляет непокрытые чтение команд, сложный I/O и save/restore, для которых нужны самостоятельные fixtures.

## Прогон Nanz/`mzv` на 2026-09-30

Внешний, проверенный по SHA-256 CZECH v0.8 Z3 теперь проходит на Nanz-интерпретаторе через `mzv`: **368 tests; 349 passed; 0 failed; 19 print tests**. Скрипт [czech-smoke.sh](../examples/zmachine/zork-west-demo/czech-smoke.sh) временно расширяет массив story до 16 КБ, подставляет абсолютный путь к проверенному файлу, проверяет сводку и характерные строки печати. Исходник и бинарь CZECH не добавляются в репозиторий. Все 19 print cases достигаются, но автоматическое сравнение всей их верстки с Frotz ещё не сделано; различаются ширина строк и поля заголовка.

[West-демо](../examples/zmachine/zork-west-demo/README.md) по-прежнему проходит девятикомандный transcript. Отдельная проверка пустого ввода подтверждает, что он больше не путается с EOF. У Dark Pit Z3 27 490 байт проверены старт, `look`, `inventory` и `restart` на `mzv`; это пока smoke, а не полный gameplay transcript. `go test -short` для `pkg/pipeline` проходит, выбранные тесты мета-функций проходят; весь `pkg/nanz` остаётся красным на известных bit-accessor/import/Z80-showcase тестах.

**Граница результата:** CZECH не покрывает persistent save/restore, потоки, экранный вывод, достаточное разнообразие входного парсинга и максимальный размер Z3 story 128 КБ. Текущий `file_read` в host обрезает файл на 65 535 байтах, а Nanz core использует 16-битные адреса. Поэтому «CZECH green» пока означает зрелость вычислительного ядра, а не полную совместимость с Z3 или готовность перенести VM на Spectrum. Следующий gate — отдельные fixtures на недостающие операции и большие истории; лишь затем Z80 trace comparison.
