# ZX приложения: исходные результаты

База81649477. Семь source/import snapshots с SHA заморожены до исполнения (freeze.json). Неизменённый `examples/abap/sap_report_zx.nanz` whole-main проходит assembled Z80 ordinary/GraceSize/GraceSpeed: весь bitmap+attributes4000..5aff по независимой модели solid cells, border OUT01FE1, без input, остановка именно source DI/HALT с SPeffe (CALL ещё не возвратился). Полная RAM/profiler WriteCount разрешают только screen и observed stack. Главная программа finite, `print_at` — неиспользованный source stub RET; никаких text-rendering/SQL результатов ей не приписываем.

`hello_zx.nanz` не возвращается и не достигает keyboard в2Mинструкций: bare counted literals передаются в NUL-consuming con_print. Это source/API defect, как прежний console_demo; отдельное исправление будет иметь новый SHA.

`zsql_mara_color_zx.nanz` — статический отчёт, не SQLite execution. Доходит HALT, но содержит32bytes font (16u16 globals) вместо заявленных91chars. Первый title character Z требует font offset464: declared model неполон. Ошибка первоначального model-test честно сохранена? Итоговый тест сохраняет explicit limitation, не объявляет rendered text PASS и не выдумывает недостающие bytes.

`zsql_zx.nanz` и `zsql_zx_real.nanz` проходят parse/compile/assemble3modes, но native SQL не исполнялся: нужен настоящий SQLite ports41..47 bridge. Непроверенный opaque ASM cross-function ABI: wrappers EX DE,HL передsqlite_exec/query, в текущей allocation sqlparam имеет memory home. Metadata/сборка не доказывает runtime. Коррекция без actual native counterexample не делалась.

Focused тест сохраняет ограничения отдельными native_scope; его общий PASS не означает успешный native whole-main у всех5apps. VM ASM=no-op здесь не oracle. Retained executable, original ASM/BIN/MIR и manifests сохранены; широкие gates не повторяли.
