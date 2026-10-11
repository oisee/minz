# Прямые u16 global loads в DE/BC

Z80 backend теперь использует direct word load для DE/BC, когда адрес —
точный global symbol, storage занимает не меньше двух байтов и адрес имеет
единственное использование во всей функции. Это заменяет byte-pair загрузку
через HL, сохраняя AF/HL и cache correctness. Числовые/общие указатели,
shared addresses, successor/inline-ASM uses сохраняют прежний путь.
Существующий прямой путь HL и memory-host permissions не менялись.

| Self-Lanz30, текущая CLI | T-states | Байты |
| --- | ---: | ---: |
| Original HIR | 1135370 | 2000 |
| Arena-first | 1135211 | 2121 |
| Append-copy | 1079795 | 1978 |
| Обе HIR-оптимизации | 1079636 | 2099 |

Прежняя сборка: original1221494 T/2091 B, both1147025 T/2197 B.
Backend сократил original на **86124 T (7.05%) и91 B**, both на **67389 T
(5.88%) и98 B**. Speed выбирает both, size append. Arena добавляет только
**159 T** на новой сборке; прежние3099 T нельзя переносить автоматически.
Оценка46608 T на двух cur loads была подмножеством, не прогнозом whole gain.

Четыре actual CLI запуска на сохранённой production сборке SHA256
`dba34fdc489d4fa00f985bff82952076f8de8f68488999dfe98364af242c9ca5`
прошли: full30speed/size, short collect и supplied-profile replay той же сборки.
Все16 actual кандидатов и4 fresh профиля проверены независимой моделью
исходника: globals, RAM snapshots, results, ordered byte events и11source
counters cumulative/per-call. Typed precompile proofs, ED opcode/root/address/
width2 descriptors, finite frames, native hashes и точный SHA JSON решения
проверены отдельно. Старые1bd и prototypeea9b контексты сохранены раздельно.
Нового per-PC профиля этой сборки здесь нет.

Focused actual regressions проверяют DE/BC words, AF/HL, destination byte cache,
store после загрузки и shared/successor/inlineASM refusal. Закрытый ED recipe
проверяет порядок low/high и rejects malformed address/width/direction.
Новый код не расширяет host permissions и не принимает произвольный ASM.

Общие проверки frozen parent: HIR22.104/MIR5.174/Nanz1.592/Lanz0.015/
C895.336/CLI21.181 s PASS; showcase31 прежнее падение,0 новых.
Первый полный pipeline достиг10m timeout и выявил три старых baseline
T/SHA assertions. Их reference обновлён по независимо проверенному BIN;
focused3tests4.173s PASS. Исходный лог сохранён. Полный pipeline повторяется
с timeout30m; итог будет добавлен после завершения.

[Прототип и его контекст](grace-global-word-load-artifacts/prototype/evidence/README.md) ·
[Текущая CLI](grace-global-word-load-artifacts/cli-production/README.md) ·
[Независимый аудит fresh профилей](grace-global-word-load-artifacts/independent-oracle/FRESH-CLI-AUDIT.md) ·
[Native CLI аудит](grace-global-word-load-artifacts/cli-native-review/FINAL_REVIEW.md) ·
[Манифест SHA256](grace-global-word-load-artifacts/manifest.json).

Все исходники, профили, решения, ASM/BIN и логи сохранены локально. Крупные
payloads используют deterministic gzip с SHA исходных bytes. Проверки
перенесённых файлов прошли без повторного CPU.

Следующий шаг: обновить четыре прежних корпуса на новой сборке, затем
отдельно проверить prefix-bounded forwarding повторной globalpos загрузки
в cur. Calls/stores/unknown effects требуют invalidation; общие purity guards
или ограничения InlineTrivial ради этого не снимаются.
