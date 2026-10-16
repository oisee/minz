# Практический C для системных библиотек Z80

Цель: исправить найденные ошибки, обеспечить честную диагностику и проверить
две реальные библиотеки на неизменённых исходниках в VM и Z80.

Baseline main — `0711d683`, compiler tree `3cd5c7bb`.
[Исходный аудит](../2026-10-16-c-standards-objc-audit.md) сохраняет контрпримеры.

1. Исправить преобразования string/character literals, comments и header names;
   сохранить исходные строки и диагностировать ошибки `#embed` с путём файла.
2. Использовать выбранную C type checker association для `_Generic`, сохраняя
   правила совместимости типов и отсутствие исполнения невыбранных выражений.
3. Устранить молчаливую подмену неподдерживаемых `_BitInt`, `constexpr` и других
   стандартных конструкций: корректная поддержка выбранного случая либо явный
   отказ. Полный C23/runtime не является обязательством этой цели.
4. Сделать ошибки corpus pipeline настоящими failures и правильно считать
   dual assertions. Новые semantic/diagnostic regressions проверять отдельно
   от старых примеров.
5. Зафиксировать и исполнить inih r62 и heatshrink v0.4.1. Upstream C/H/license
   сохранены без изменений в `research/system-c-validation/upstream`; SHA и
   точные commits — в manifest. Target headers, build configuration и обвязка
   отдельны от библиотек. Freeze workloads и независимые ожидания до проверки;
   сохранять ошибки, raw/final VM, assembled Z80, память и PC/SP.
6. Пройти подходящие core/integration проверки, опубликовать фактическую матрицу
   поддержки и ограничения, интегрировать исправления и доказательства в main.

Статус: цель завершена. Пункты 1–6 интегрированы через PR #187 в main
`936749f61`. Compiler tree `7fe5805c` совпадает с проверенным финальным checkpoint.
Неизменённые inih и heatshrink прошли default/Size/Grace: 45 native и 90 raw/final
VM проверок. Финальный CI: все required checks и lint PASS, 435 fast pipeline
entries в трёх успешных shards. Проверен data-only C import из Nanz.
Пять прежних CLI profile/selector failures воспроизведены на baseline и текущем
коде с одинаковыми результатами; они перечислены в отчёте и здесь не исправляются.
Следующие frontend hardening предложения, включая ABAP native Go parser seam,
собраны в [новом отчёте](../2026-10-16-c-and-frontends-hardening.md).
