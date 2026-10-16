# Что можно узнать у SDCC: два upstream-теста на общем стенде

16 октября 2026 года. Это небольшой black-box опыт: одинаковый C проходит через
MinZ и SDCC, после чего сравниваются результат, память, размер и такты Z80.
Алгоритмы двух выбранных тестов не переписывали. Обе программы корректно прошли
все десять сборок, но SDCC оказался меньше и быстрее. Это полезные проигрыши:
полученный ASM показывает общие места для следующей работы.

## Откуда взяты тесты

В официальном source release SDCC 4.5.0 каталог `support/regression/tests`
содержит 1514 файлов `.c` и 111 шаблонов `.c.in`. Regression harness проверяет
ошибки исполнения; `get_ticks.py` и `collate-results.py` также собирают загруженные
байты и такты симулятора. Это прежде всего проверка корректности. Отдельно в
`support/tests/dhrystone` есть Dhrystone для Z80; здесь его не запускали.

Источник — официальный [архив SDCC 4.5.0](https://sourceforge.net/projects/sdcc/files/sdcc/4.5.0/).
SHA архива и каждого выбранного файла находятся в
[method.json](copy-fat12-optimization/sdcc-upstream/method.json).
Измеренный MinZ — `02d045bd`, compiler tree `3cd5c7bb`; тот же compiler tree
интегрирован в main `4c79b1f8`. Это отдельный опыт, не новый запуск всего regression suite.

## Общая обвязка и проверки

До замера выбраны `bitwise8reg.c` и `gcc-torture-execute-loop-11.c`.
Вместо оригинального `testfwk.h` использован общий ASSERT-адаптер со счётчиками
в памяти. SDCC выполняет C99 preprocessing; полученный замороженный translation
unit побайтово одинаков для обоих компиляторов. Исходные upstream-файлы не изменены.
Первый C89 preprocessing отказал из-за `//` в исходных директивах; ошибка сохранена.
Этот опыт не доказывает допуск этих файлов как строгого C89.

Каждая сборка вызывает один полный test entry. Проверяются результат, счётчики
и перечисленные области памяти; массив из 199 `int` заранее заполнен A5 и затем
сверяется побайтово с независимым ожиданием 0..198 в little-endian формате.
Native Z80 проверяет PC/SP; MinZ дополнительно проходит raw и final MIR VM.
Измерения включают ASSERT-адаптер, поэтому это стоимость regression-теста,
а не изолированного алгоритма. Полный 64-КБ trace здесь не проверялся.

Статический массив SDCC не экспортировал в linked NOI. Для наблюдателя его адрес
восстановлен из object SYM и базы linked DATA с проверкой границ. Исходные бинарник
и NOI остались неизменны; производная карта и SHA сохранены. Первые ошибки
наблюдателя остаются в [initial-summary.json](copy-fat12-optimization/sdcc-upstream/initial-summary.json).

## Результаты

| Тест | Компилятор / режим | Полный модуль, байты | Такты |
|---|---|---:|---:|
| bitwise8reg | MinZ default / Size | 387 | 1578 |
| bitwise8reg | MinZ Grace | 370 | 1503 |
| bitwise8reg | SDCC Size | **159** | **606** |
| bitwise8reg | SDCC Speed | 160 | **606** |
| Обратный цикл | MinZ default / Size / Grace | 614 | 124104 |
| Обратный цикл | SDCC Size | **522** | 52467 |
| Обратный цикл | SDCC Speed | **522** | **52038** |

Итого **10/10 PASS**. Среднее между разными нагрузками не вычисляем;
результат не распространяем на остальные upstream-тесты.

## Какие уроки видны в ASM

В байтовом тесте MinZ расширяет промежуточные значения до слова, затем усекает
и снова расширяет для AND. SDCC оставляет байтовые значения в регистрах:
`LD A,(c0); LD C,A; INC C`, затем байтовые AND/OR. Здесь нужен общий анализ
наблюдаемой ширины выражений, а не специальное правило под имя теста.

В обратном цикле SDCC держит счётчик в DE и завершает его через
`DEC DE; BIT 7,D; JR Z`. Наш новый counted-pointer проход рассчитан на другой
доказанный случай: возрастающий байтовый индекс. Для знакового countdown нужны
отдельные факты диапазона и эквивалентности адресной рекуррентности.

Эти улучшения ещё не реализованы. Grace немного улучшает первый тест,
но не компенсирует отсутствующие общие правила сужения и адресации.

## Посмотреть исходники и настоящий ASM

- Байтовый тест: [оригинальный C](copy-fat12-optimization/sdcc-upstream/bitwise8reg/bitwise8reg.c),
  [MinZ Grace ASM](copy-fat12-optimization/sdcc-upstream/bitwise8reg/minz/grace/program.a80),
  [SDCC Size ASM](copy-fat12-optimization/sdcc-upstream/bitwise8reg/sdcc/size/program.asm).
- Обратный цикл: [оригинальный C](copy-fat12-optimization/sdcc-upstream/gcc-torture-execute-loop-11/gcc-torture-execute-loop-11.c),
  [MinZ ASM](copy-fat12-optimization/sdcc-upstream/gcc-torture-execute-loop-11/minz/default/program.a80),
  [SDCC Speed ASM](copy-fat12-optimization/sdcc-upstream/gcc-torture-execute-loop-11/sdcc/speed/program.asm).
- [Все результаты](copy-fat12-optimization/sdcc-upstream/summary.json) и
  [манифест](copy-fat12-optimization/sdcc-upstream/manifest.json).

Полные бинарники, наблюдения, первоначальные ошибки и воспроизводящий local driver:
`/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/copy-fat12/sdcc-upstream-pilot/`.
Этот исследовательский набор не копировался на игровую шару.

[Основной бенчмарк C/Nanz/SDCC](2026-10-16-c89-nanz-full-benchmark.md) ·
[Интегрированные оптимизации копирования и FAT12](2026-10-16-copy-fat12-results.md).
