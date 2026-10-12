# ZX apps и сохранение whitespace в assembler

Исходный пакет0f1d8e0a сохраняет exact sources и результаты до исправлений. Неизменённый sap_report_zx проходит whole-main screen model, borderOUT01FE1 и sourceHALT/SPeffe: ordinary2103B80295T; GraceSize/Speed2120B80312T. Grace тут увеличивает17B/17T, выигрыша нет. Неиспользованные poke/print_at не являются whole-main oracle; print_at — original stub.

Source fix514d2ebf меняет только17 con_print literals hello_zx на явные C strings. После него independent oracle поймал вторую, production ошибку: ASM `DB "CPU:      ",0` содержал6пробелов, assembler BIN сохранял1. a6f0d5e2 сохраняет хвост операндов после mnemonic; quoted bytes не проходят через Fields+Join. Unit DB tests покрывают spaces/tabs, single/double quotes, label и multioperand. Остальное поведение parser не расширяли (например quoted semicolon остаётся отдельной прежней проблемой, не проверенной здесь).

Исправленный hello_zx whole-main проходит independent full bitmap+attributes, фиксированную fontfixture, точные выравнивающие пробелы и17keyboardINFE requests. Нажатие lowercasea используется как ответ на ожидание клавиши; source его не печатает. Cursor row14col0, actualCALL return7003/SPf000, observed stack16B. ПолнаяRAM/profiler WriteCount разрешают только screen+3mutableconsoleglobals+observedstack; code/literals/font/readonlytables не изменяются.

| Режим hello | BIN | T-states whole-main |
|---|---:|---:|
| ordinary |2442|563972|
| GraceSize |2435|564021|
| GraceSpeed |2435|564021|

Grace −7B/+49T: ускорения нет. Focused all15subcases PASS2.537s; это native positive только SAP+correctedhello. Fullz80asmPASS0.36s. MARA color source заканчиваетсяHALT, но declared font32B недостаточен; не объявлен displayPASS. SQL variants compile/assemble, не nativeSQLproof: нужны настоящий SQLite port bridge и проверка hardcoded wrapper ABI. VMOpAsm=no-op не oracle, физическийSpectrum/CPM не заявлены.

Retained before assembler ASM/BIN/screen/model/log сохранены отдельно от selected result. Exact original snapshots, corrected source SHA/freeze, ASM/BIN/MIR/results и retained executables входят в манифесты. Broad pipeline/CLI не повторялись.
