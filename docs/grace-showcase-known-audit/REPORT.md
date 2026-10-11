# Аудит known-skipped showcase

База `fdb26b92`, 192 файла. До аудита: 167 PASS, 25 SKIP, 0 FAIL. Это не означало исправление всего корпуса.

Теперь `MINZ_SHOWCASE_CHECK_KNOWN=1 go test ./pkg/nanz -run '^TestShowcaseCompileAssemble$' -count=1 -v` выполняет известные пропуски. Каждый case печатает JSON с точным repository-relative source path, Go case name, status и достигнутой стадией. Исключения привязаны к точным путям вместо base filename.

Все исключения включены: **186 PASS, 6 FAIL, 0 SKIP**, exit 1. Из прежних 25 пропусков **19 проходят**; их obsolete skip записи удалены. Обычный прогон после очистки: **186 PASS, 6 SKIP, 0 FAIL**, exit 0. Compiler code между прогонами не менялся.

Коллизии старого map: `01_sum_array` скрывал Nanz и PL/M, `factorial` — Lizp и Pascal, `showcase` — рабочий PL/M из-за ошибки Lizp. Теперь это исключено; focused collision cases также проверены отдельно.

| Source | Стадия | Реальное препятствие |
|---|---|---|
| reports/showcase-src/2026-03-10/ex4a_abs_diff_iar.nanz | MIR2 runtime assert | signed abs_diff(20,-10)=226 вместо 30; старый «IAR unsupported» ярлык неверен |
| reports/showcase-src/2026-03-15/ex34_tetris.nanz | HIR verifier | main non-address type u8; текущий examples/zx/tetris.nanz проходит |
| examples/lizp/showcase.lizp | HIR verifier | quoted ASM становится unknown variable в zx_poke |
| examples/lizp/zx_checkerboard.lizp | HIR verifier | тот же quoted ASM frontend gap |
| examples/lizp/zx_rainbow.lizp | HIR verifier | тот же quoted ASM frontend gap |
| examples/zmachine/zork-west-demo/zvm.nanz | native codegen | caller spill save unsupported width4; invalid native u32 spill loads |

`classification.json` содержит точные сообщения и ссылки на семантику. Language Book v7 §3.2 требует signed i8 comparison; §2.15/3.3 описывает адреса; §3.1 заявляет u32 через shadow pairs. Эти ошибки не объявляются просто «неподдерживаемой программой». У ZVM отдельно нужен native runtime для file/UTF8/input externs; починка spill не докажет работу всей программы. Book описывает Nanz ASM, а не quoted Lizp ASM: этот случай отмечен как frontend syntax gap.

PASS здесь означает source → HIR → MIR2 → ASM → BIN и только содержащиеся в исходнике assert, если они есть. Полное игровое исполнение Tetris/plasma, полноценное sieve состояние, CP/M/Agon OS и все программы на железе не проверялись. Число 186 не означает «186 программ полностью исполняются корректно». Шесть ошибок не исправлялись в audit commit, ни verifier, ни ожидания не ослаблены.

Сохранены bypass лог, 25 original-known paths, 192 all-case rows, normal-after лог, default-case rows, classification и focused collision лог.
