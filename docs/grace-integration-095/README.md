# Историческая weighted shape и свежий showcase

Snapshot ba697cbd. Explicit quotient importer использует ENRT4v record5,
прочитав172байта. Он объединяет только принудительно одинаковые physical
byte locations без внутреннего live edge для поиска предложения. Все13
исходных SSA values/Ops/save/return и full current context сохраняются;
назначение поднимается обратно и проверяется через AddChecked.

Root all256 independent raw/finalVM/native helper+caller PASS2.412с:
original44→20B, helper182→86T, caller238→142T. Изменённый factor9 использует
ту же historical запись с новой full checked key:40→19B/166→82T/222→138T.
Это параметрическое повторное использование назначения, не таблица значений
умножения и не доказательство оптимальности всех shape assets.
[Конвертер и границы](../../reports/2026-10-11-weighted-historical-quotient.md).

Полный VIR на этом snapshot PASS (точное время в логе); import-enrt CLI
собирается. Архив reopened, все29 payload SHA сверены и сохранены на шаре.
Fresh showcase compile/assemble после context-once и lane identity:
PASS12.711с, 193PASS/1SKIP из194. Он предшествует quotient, который не меняет
default compiler. Это compile gate; найденный packed FAT12 native mismatch
остаётся отдельной текущей correctness задачей.
