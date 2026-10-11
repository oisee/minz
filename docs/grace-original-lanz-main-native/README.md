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

## Сохранение frozen provenance после исправления текущего исходника

Source fix меняет SHA текущего примера. Исторический prospective freeze остаётся SHA `0e92689c6bb4a92fd681c9bc70d5d50549631a0fdd669cd026f723cf4e197526`; его snapshot уже сохранён в `docs/grace-parser-prospective-heldout/artifacts/source.nanz`. Исторические `TestParserProspectiveHeldout`, shared memory-host fixture, policy export/reference tests теперь читают **этот неизменяемый snapshot**, сохраняя прежний declared SourceName и workload. Freeze, старые manifests и capture outputs не изменены. Новые exact current-source baseline/main tests продолжают читать `examples/nanz/self_lanz_parser.nanz`. Каталог явно помечает prospective relation как historical snapshot, не выдаёт его за proof нового SHA.

Аудит также нашёл три давно устаревших hardcoded backend references: 1 291 565 T / `a9f98...`. При нынешнем компиляторе на **том же frozen source** независимый full-state/events oracle подтверждает 1 269 009 T / 2364 B, SHA `27a7c8ea7b77725ee5f61e9f4be625bd1b582b1b03e1013021e0ab3766ee8545`. Это не следствие CString main fix. Current fixed-cost assertions обновлены по реальному запуску; прежние evidence не изменены. Target/factory/association rerun PASS 3.338 s; их exact source/ASM/BIN/observations сохранены в `frozen-source-current-backend/` с SHA. Export helper теперь сохраняет правильный frozen source вместо текущего файла рядом со старым evidence.

Prospective rerun PASS 21.732 s: heldout baseline 2 141 572 T / 2364 B, selected mask2 2 093 170 T / 2344 B, прежние binary SHA. Нагрузки/выбор и independent model не переписаны. Три optional export tests остались с прежними явными environment gates; их skips не выдаются за исполненные export searches. `historical-rerun.txt` сохраняет первоначальное обнаружение stale-reference failure, последующие успешные проверки — `historical-reference-rerun.txt` и `prospective-preserved.txt`.
