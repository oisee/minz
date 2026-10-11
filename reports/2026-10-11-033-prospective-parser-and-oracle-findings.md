# Self-Lanz: заранее отложенные входы и новые находки оракулов

Training/heldout inputs зафиксированы до сбора профиля коммитом c533cf10 (в объединённой ветке 605f7e80). Неизменённый self_lanz_parser.nanz; фактический API использует ContractOpt=false, что явно записано в freeze. Четыре training маски измерены; pure Speed выбирает append-only mask2, arena peeling на этих входах проигрывает.

Training 1016414→992049 T, 2364→2344 B. На шести дисjoint heldout формах тот же BIN: 2141572→2093170 T (−2.26%),−20 B. Независимые Go/Python проверяют дерево, все source/node/string arenas, globals и порядок обращений к памяти. Heldout selector не вызывался. На объединённой ветке TestParserProspectiveHeldout PASS, 23.562 s; переносимый Python audit повторно PASS. Все 37 файлов пакета скопированы на шару и SHA256 проверены.

Добавлена независимая сквозная проверка wide global initializers: u24/u32 и массив сохраняют точные байты после исполнения функции VM/Z80, включая переполнение слова. Исправлено ложное предупреждение label audit для node_tag EQU peek; assembler-admitted alias regression проходит.

Проверка поведения Pascal bubble sort выявила настоящий native miscompile: Z80 делал один проход, VM сортировал весь массив. Причина — OpNot игнорировал реальные Src/Dst и использовал CPL над A; исправление проходит full-array oracle в worktree подагента и ещё не интегрировано.

Production table integration в работе: обычный CompileHIRSteps, явный Size opt-in, только полностью связанные local leaf contracts/RET; исключение unchecked peepholes, baseline fallback. Первичный 256-case runtime add5 даёт8 → 6 B с сохранением поведения; окончательная CLI/integration проверка ещё впереди.

Полный pipeline на cb587fe4 подтверждён живым session 2903 и пока не завершён. Это не PASS. Большая цель остаётся активной: поведение обещанного корпуса, production checked tables и остальные полезные возможности инвентаря.
