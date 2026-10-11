# Agon MOS: поддерживаемый Nanz console API

База: `0717d100`. Добавлен отдельный `stdlib/agon/mos.nanz`; большой старый `mos.minz` не изменён. Nanz resolution предпочитает `.nanz`, поэтому старый `.minz` больше не передаётся Nanz parser.

В `hello_world.nanz` удалены semicolon из старого MinZ синтаксиса, C-string literals явно записаны как `c"..."`, команда компиляции указывает `-b ez80 -t agon`. Поведение: greeting → blocking getkey → echo → CRLF → goodbye.

Минимальная Nanz библиотека содержит только используемые console функции:

- `mos_putchar`: `RST.LIL $10`, A=character.
- `mos_puts`: HL=address, BC=0, A=0 перед `RST.LIL $18`. Это delimited NUL режим MOS. Bare SString здесь непригоден.
- `getkey`: A=0 перед `RST.LIL $08`. Старый mos.minz использует неверный vector 0: upstream vector 0 reset, vector 8 dispatch.
- `newline`: character 13, затем 10.

Для ASM контрактов явно задан `(clob all)`: compiler сохраняет живой key через последующий puts. Импорт не объявляет мнимую поддержку filesystem/RTC/24-bit pointers.

## Доказательства

`TestShowcaseCompileAssemble/hello_world.nanz`: PASS до BIN. Это сборка, не Agon runtime.

`TestAgonHelloMOSProductsAndBoundary`:

- Обычный Z80 caller + явный mock границы MOS: PASS, BIN 181 bytes, все output bytes совпадают с независимым ожидаемым текстом, ровно один getkey (возвращает Q), key сохранён через puts, PC возврата $7000 и SP=$FFF2.
- eZ80 ADL assembly/BIN: PASS, 197 bytes, ORG=$040045, `.ASSUME ADL=1`. Сохранены реальные ASM/BIN; encoding `RST.LIL` проверен как `5B CF`, `5B D7`, `5B DF`.
- `pkg/z80asm` и `pkg/ez80`: PASS.
- `pkg/nanz -short` всё равно включает full showcase; FAIL только на `sqlite_test.nanz` (unknown SQLITE_ROW) и `zsql_zx.nanz` (invalid LD), перечисленных в сохранённом логе. Полный зелёный Nanz пакет не заявляется.

Mock специально перехватывает `5B RST` до Z80 CPU и выполняет только API boundary. Он не притворяется, что обычный Z80 выполняет eZ80 ADL prefixes. Native caller instructions выполняются настоящим repository Z80 emulator. Время mock не является замером Agon T-states.

Upstream proof сохранён локально: `src_startup/vectors16.asm` подтверждает reset vector 0, dispatch vector 8 и правила BC/A у string vector 18; `src/mos_api.inc` подтверждает getkey=0. Источник https://github.com/breakintoprogram/agon-mos (main, snapshot 2026-10-11).

## Границы

Current pipeline `Backend=ez80` оборачивает Z80 assembly заголовком ADL; отдельный `ez80.MIR2Codegen` здесь не используется. Проверка сборки 197-byte продукта не доказывает полную корректность ADL ABI (24-bit allocation/spill/stack), реальную прошивку MOS, блокирующий keyboard, MOS header launcher и возврат ОС на hardware. Это следующий target-specific этап. Legacy mos.minz ошибки не исправлялись, чтобы не объявлять непроверенную migration всего API.
