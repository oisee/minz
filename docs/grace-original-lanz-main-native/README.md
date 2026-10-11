# SelfLanz: полный main исходного примера, native и CP/M runtime adapter

База `c3782f21`. Закрыт реальный пробел whole-main: предыдущие 30 computational workloads проверяли оригинальные parser entries, а исторический CP/M capture использовал отдельный CString-derived source. Теперь исполняется **целый main текущего `examples/nanz/self_lanz_parser.nanz`**, включая загрузку строки, parse, recursive printing и статистику.

## Ошибка исходного примера и минимальная правка

До изменения `main` помещал обычный `"..."` literal в `^u8`, затем читал до NUL. По Language Book v7 §18.1 plain literal — SString `[u8 length][bytes]`, CString `c"..."` — `[bytes][0]`. До правки независимый native оракул обнаружил:

```text
Parsed: -
Nodes: 1 Strings: 2 bytes
```

Вместо 17 узлов и 24 intern bytes; source/node/string RAM расходилась. Копирование начиналось с length byte 41 (`'-'`) и не имело CString termination. Это **ошибка примера**, не miscompile: компилятор правильно реализовывал специфицированный формат строки.

Исправлена только строка `let src: ^u8 = c"..."` и добавлена ссылка на Book. Parser/interner/printing алгоритмы и исходный `main` не переписывались. Старые source/ASM/BIN/stdout/fullcapture сохранены под `before/`; исторические публикации не перезаписаны.

## Что реально проверено

Независимая Go-модель парсит fixture и самостоятельно строит узлы/связи/interning, затем рендерит **модельное дерево**. Ожидаемый stdout не получается из компилируемого print_node. После выполнения проверяются:

- Все 135 байтов stdout, включая LF, Input/Parsed/Nodes/Strings.
- Все 4096 байтов node arena, 8192 string arena и 4096 source buffer, включая неиспользованный хвост.
- Все 10 globals: pos/src_len/node_bump/str_bump/base pointers/tags; 17 узлов и 24 intern bytes.
- Границы соседних областей, stack canaries и исходный SP.
- Каждый неизменяемый байт assembled image: код и literals. Исключены только явно именованные source globals, wrapper status/SP и exact declared compiler spills; произвольное изменение кода не допускается.

Оба режима собирают текущий **один и тот же source** и исполняют его весь `main`. Экстерны peek/poke получают явные native реализации с проверенным ABI `HL`/`C`/`A`.

| Режим | T-states | Размер | SHA256 BIN |
|---|---:|---:|---|
| Обычный generated native OUT 0x23 | 204 560 | 2410 B | `3adb9988abe42dbe66b53194ee3307f7be4ea79965cfaffcf2cd456ba39dc47f` |
| CP/M BDOS2 assembly runtime adapter | 229 769 | 2477 B | `e3792f5020d796e509a223cab661e71fa53552577786ddb78b4d1a7ee559a304` |

Native default main завершает DI/HALT со SP 0xeff0. CP/M образ начинается на 0x0100: wrapper сохраняет incoming SP, задаёт отдельный stack, вызывает original main, пишет status1, восстанавливает incoming SP и возвращается по CP/M warm-boot PC0. Итог PC0/SP0xfe00/status1. Main labels/branches/body сохранены.

CP/M runtime adapter **явно** переводит generated `OUT (0x23),A` в `CALL __oracle_bdos_putchar`, сохраняющий AF/BC/DE/HL/IX/IY и вызывающий BDOS function2 через CALL5. Process termination DI/HALT переводится в RET для wrapper. Реальный emulator CALL5 handler получает каждый выводимый байт; тест намеренно меняет A/HL при BDOS return, чтобы отсутствие сохранения регистров не маскировалось.

Это **test-only adapter-derived executable из exact current source**, не production `--target cpm` флаг и не реальная CP/M машина. Go host BDOS work исключена из CPU T-states. Отдельный adapter объясняет различия образа; программа/AST/алгоритм не меняются для CP/M. Исторический derived CString COM остаётся отдельным доказательством с собственными SHA.

## Повторная проверка parser workloads

`TestOriginalSelfLanzParserBaseline` повторно прошёл 30 независимых вызовов после source fix: computational результат прежний, 1 269 009 T; полный образ теперь 2410 B. Изменение literal затрагивает только main. Каталог связывает новый full-main test с текущим source SHA, а прежние historical artifacts не переписывает. Число покрытых путей не растёт от второго режима одной программы: по-прежнему 23 bounded-covered из 193.

## Воспроизведение и артефакты

Из `minzc/`, Go 1.26.8:

```sh
MINZ_ORIGINAL_LANZ_MAIN_ARTIFACTS=/absolute/evidence go test ./pkg/pipeline -run '^TestOriginalSelfLanzMainNativeFullState$|^TestOriginalSelfLanzParserBaseline$' -count=1 -v
MINZ_SHOWCASE_ORACLE_CATALOG=/absolute/catalog.json go test ./pkg/nanz -run '^TestShowcaseExecutableOracleCatalog$' -count=1 -v
```

Focused main+baseline PASS за 3.108 s. Полный showcase/pipeline этим этапом не запускался. В `port23/` и `cpm-bdos2/`: exactsource, fullASM/BIN, stdout, capture и независимый oracle. JSON gzip без потери. `manifest.json` хранит SHA256 всех payloads, кроме себя. `before.txt` фиксирует реальный old-source failure.
