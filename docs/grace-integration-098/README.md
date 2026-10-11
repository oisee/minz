# Historical shape coverage и второе FAT12 исправление

Root snapshot3054481f: coverage audit ff4b2afa, liveHL shift fix3054481f.
Повторён полный аудит12 exactпрограмм/118функций: PASS19.091с.
9unique admitted:3smaller/4equal/2larger. Larger не выбираются.
Candidate native сравнивается с rawVM, это дифференциальный oracle;
самостоятельные host arithmetic models weighted/triple/low_nibble сохранены
в предыдущих пакетах. 67074tuples на каждый baseline/candidate путь.
[Scope](../../reports/2026-10-11-indexed-shape-corpus-coverage.md).

Parametric source/name/immediate/register remapping root PASS:
VIR0.026с/pipeline0.147с. Историческое assignment повторно используется,
но каждое changedsource получает новую полную checkedkey.
256MiB prefix read limit отличается от физического truncation.
Coverage archive reopened,97payload SHA проверены и сохранены на шаре.

LiveHL сохраняется при constant word shift черезIX/IY. Root180physicalnative
cases PASS0.211с. Это второй конкретный bug послеBIT matcher; defaultFAT
packed нагрузка проходит, GraceSize cond-return flags исправляются отдельно.
[Shift report](../../reports/2026-10-11-067-indexed-word-shift-live-hl.md).
Не объявляем полный FAT PASS или новый measured heldout winner.
