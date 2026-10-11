# Interning: совместный выбор copy и weighted checksum до Z80

Typed port подсистемы self_tokenizer ищет Pascal string records, переиспользует совпавшую строку и размещает новую при промахе. Обычный intern_checked caller затем проверяет сохранённые байты через weighted checksum и обновляет persistent counters. Source и arena — перемещаемые resources; bump — u16 offset. Исходный extern tokenizer не изменён и целиком не проверен этим этапом.

30 последовательных вызовов дают15 reuse hits и итоговый bump559. Корпус включает пустые/максимальные255-byte строки, mismatch в первом/среднем/последнем байте, разные длины, prefixes, повторное использование и modular checksum/counter wrap. Независимый host oracle выполняет поиск и последовательное размещение Pascal records; все кандидаты проверяются по каждому результату, полному состоянию и точным чтениям/записям до/после compilation и на собранном Z80.

Новый API MeasureMixedCopyWeightedVariantSets и CLI --hir-profile-mixed-copy-weighted-sites --hir-profile-joint --hir-profile-auto DIR --hir-profile-auto-workload workload.json устанавливают copy/scalar/pointer choices одновременно до одного seeding lineage. Последовательно соединять прежние selectors нельзя. Copy site и weighted site дают3 independent bits и8 кандидатов; лимит4choice dimensions/16кандидатов. Constant-if и остальные правила выключены; eligible source proof остаётся точным. Профиль v5 выбирает наблюдавшиеся sites и проверяется controlled replay, без cost/guard extrapolation.

Source/pre/post validators проверяют обе kernel формы и неизменность всех остальных функций/module definitions. У каждого фактического цикла обязательна метка; метки на других statement kinds запрещены. Unmapped source loops отклоняются; original loops в неизменённых функциях привязаны к своим исходным HIR paths. Independent review обнаружил перенос метки на return и обмен метками обычных циклов; сохранённые reproducer и before/after подтверждают отказ исправленного validator.

Стоимость полного intern_checked workload:

| Copy | Scalar | Pointer | T-states | Bytes |
|---|---|---|---:|---:|
| off | off | off |1635072|626|
| off | off | on |1184776|758|
| off | on | off |1089098|539|
| off | on | on |992839|691|
| on | off | off |1604370|761|
| on | off | on |1154074|893|
| on | on | off |1058396|674|
| on | on | on |962137|826|

Speed выбирает все3преобразования: −41,2%T. Size выбирает scalar-only: −87B. Copy сам по себе даёт небольшой выигрыш: reuse calls уже не копируют строку. Победители сохраняются как фактически измеренные бинарники; отдельная mixed decisions schema хранит3карты, pre/post proof и binary/evidence SHA256.

Исходный target baseline обнаружил backend ошибку: word spill-to-spill copy затирала живой checksum pointer в HL. Direct и parallel copies теперь сохраняют HL; TSMC destination также сохраняет AF. Девять машинных регрессий проверяют pending shuffle sources, carry, A/HL и stack. Исходная ошибка сохранена отдельно от исправленного baseline. Это измерения assembler binary в Z80 emulator; настоящее железо остаётся следующим уровнем проверки.

[Артефакты и независимый аудит](grace-pgo-mixed-intern-artifacts/README.md).

Финальный pipeline226.856sPASS (budget отдельно722.080s на предыдущем коммите; все4budget ASM и assembled binaries до/после spill-fix совпадают побайтно; оба прохода Grace воспроизведены, OptSize эквивалентность проверена по пути кода), HIR/MIR2/Nanz7.930/3.949/1.775sPASS, profileCLI25.445sPASS; showcase31existingfailures0new. Независимый аудит2880candidatecalls и4Go evidenceSHA256 прошёл.
