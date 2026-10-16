# C89: аудит условий завершения

16 октября 2026 года. Цель активна. Main c69f2197 включает PR 163–169;
PR 170 ожидает строгого CI. Этот документ не объявляет PR170 интегрированным.

| Условие objective.md | Доказательство | Остаток |
|---|---|---|
| 1. Исполняемая матрица и честные ограничения | feature-matrix.md: именованные native/VM семейства, отдельные admission tests | Обновить окончательные main/PR ссылки после интеграции |
| 2. Integer, выражения, control, storage, pointer, aggregate, qualifiers | Integer/pointer/lvalue/scalar/storage/volatile/enum; language admission; control matrix и closure | Control closure интегрирована в PR169; полное ISO не заявляется |
| 3. Независимые raw/final VM и собранный Z80 | Checked-in C; независимые Go модели; строгий GCC для переносимых результатов; память/events/PC/SP в control/effects и storage tests | Последние native width/closure проверены локально, дождаться CI |
| 4. Реальные multi-argument/result slots и цена | Pair и width matrix, no alloca; transfer-cost primary/heldout; fallback guards | Width boundary пока PR170 |
| 5. Copy/lifetime независимо от выбора | aggregate parameter mutation/direct/indirect/recursive; return-lifetime owned buffer; multiply overlap regression | Интегрировано в PR166, main подтверждён |
| 6. Редукции, heldout и разделение fast/deep | Минимальные fixtures в testdata, forced allocation native tests, frozen GCC expression oracle; deep manual/nightly | Truncation regression интегрирован через PR169; final width integration proof |
| 7. FAT12, Nanz/C, строгий CI, main | main548 full C89 PASS52.642s; image-backed FAT12 default/Size: 1537 bytes, 10 sectors, точное совпадение; linked-import native suites | Merge170 с 6 обязательными зелёными checks и повторить representative main proof |

Заявленный допуск — системный embedded C с char8, short/int/pointer16, long32
на Z80. Без floating point, bitfield и runtime varargs. У hosted libc,
полного preprocessing, неизвестного indirect ABI и всего ISO C89 нет обещания
полной реализации. Long long и VLA — C99 и явно отклоняются где unavailable.
Нельзя использовать успешный разбор корпуса как доказательство семантики.

Декомпозиция ограничена shallow scalar structures до четырёх байт;
overlapping/nested/volatile/escaping и косвенные границы используют память.
При сравнении стоимости проверяется весь вызов вместе с caller transfers.
Измеренные primary 193→90 bytes / 56301→23569 T и heldout
255→141 bytes / 100012→46138 T относятся только к этим двум fixtures.

После интеграции PR170 следует обновить конечные коммиты и CLI команды
в этом аудите, затем повторно проверить все семь условий. До этого цель
остаётся активной. Чистый main c69f2197 подтвердил control-closure и native
truncation regression: C89 0.384 s / MIR2 0.009 s, PASS.
