# MinZ — статус и дерево бэклога

**Актуализация 2026-09-30:** главная ветка переименована в `main`; снимок до уборки сохранён в `main-archive` на `7cee4aae`. [Карта веток, ссылок, субмодулей и CI](../../reports/2026-09-30-Repository-Cleanup-and-Branch-Map.md) описывает изменения после исходного аудита ниже. A1/A2 выполнены: `PR gate` прошёл [в PR #20](https://github.com/oisee/minz/pull/20), `main` защищён обязательными PR и актуальной проверкой. Пять устаревших workflow удалены; до пересборки автоматизирован только CI/PR gate. Полный набор тестов остаётся отдельной открытой задачей A3/A5.

**Релиз и Z-machine:** [проверка зрелости Nanz и инвентаризация артефактов](../../reports/2026-09-30-Release-and-ZMachine-Maturity-RU.md) добавляет трек J. Это экспериментальный E2E gate после A3/A4/C3, а не условие ближайшего correctness release.

Дата: **2026-09-19**. База: **`f51171a5`**, `master`, после `git fetch origin` совпадает с `origin/master`.
Это текущая очередь решений и работ; старые планы сохраняют исторические детали. Расширенный [аудит MinZ, gpuforce, Z80/6502 optimizer и архивов](../../reports/2026-09-19-Ecosystem-Audit-RU.md) содержит свежие проверки и ограничения.

Свежий [глубокий технический аудит](../../reports/2026-09-19-Deep-Technical-Audit-RU.md) уточняет приоритеты: GPU witness race, aliases, dense indexing, reader contracts и AY register semantics. Исправления ещё не внесены.

## Где проект сейчас

**Основная перспектива — надёжный Nanz/C → MIR2 → Z80 с измеряемой оптимизацией и удобным полным циклом compile → assemble → execute.** Общий IR, PFCCO (выбор соглашений вызова), compile-time asserts и готовые инструменты дают сильную основу. Главный ограничитель сейчас — достоверность проверок и корректность исполнения, а не количество языков.

| Область | Где смотреть | Состояние и перспектива |
|---|---|---|
| Production-компилятор | [CLI](../../minzc/cmd/minzc/main.go), [pipeline](../../minzc/pkg/pipeline/pipeline.go), [MIR2](../../minzc/pkg/mir2) | PBQP + Z80 codegen; PFCCO остаётся на production-пути. Лучший кандидат на ближайший correctness release. |
| Языки | [Nanz](../../minzc/pkg/nanz), [C](../../minzc/pkg/c89), [Frill](../../minzc/pkg/frill), [Lizp](../../minzc/pkg/lizp), [Lanz](../../minzc/pkg/lanz), [Pascal](../../minzc/pkg/pascal), [PL/M](../../minzc/pkg/plm), [ABAP](../../minzc/pkg/abap) | Широкая поверхность, неодинаковая готовность. Nanz/C — предлагаемый поддерживаемый минимум; остальные получают явную матрицу возможностей. |
| Инструменты и runtime | [cmd](../../minzc/cmd), [stdlib](../../stdlib), [примеры](../../examples), [fun](../../fun) | MZA, MZE, MZV, MZX, debugger/LSP и платформенные библиотеки уже имеют код; наличие команды не доказывает готовность сценария. |
| Исследования | [VIR](../../minzc/pkg/vir), [LIR](../../minzc/pkg/lir), [oracle](../../minzc/cmd/vir-oracle), [research](../../research) | VIR убран из production, `--vir` удалён. Offline oracle и таблицы сохранены; LIR экспериментальный. Физическое разделение shared-кода и solver ещё отдельная задача. |
| Другие targets | [LLVM](../../minzc/pkg/mir2llvm), [GPU](../../minzc/pkg/mir2gpu), `minzc/pkg/mir2{c,go,qbe,wasm}` | Рассматривать по отдельным возможностям: генерация, сборка, исполнение, asserts. В LLVM/GPU найдены ветки, выдающие TODO вместо ошибки. |
| Документация и поставка | [README](../../README.md), [STATUS](../_archive/root-legacy/STATUS.md), [snapshot](../_archive/root-legacy/COMPILER_SNAPSHOT.md), [CI](../../.github/workflows/ci.yml) | Статусы расходятся; STATUS/snapshot датированы 2025. CI всё ещё требует Go 1.20/1.21, root npm и отсутствующие `deps/build` в minzc Makefile при Go 1.24 в модуле. |

### Что проверено в этой сессии

- `cd minzc && go build -o /tmp/minz-backlog-mz ./cmd/minzc` — **успешно**.
- `make test-quick` завершился: 46 `ok` packages, 4 failing packages (ABAP, Nanz, VIR, z80testing), 21 top-level failing test. **При этом make вернул 0.** ABAP отдельно подтверждён как missing dependency; showcase зависит от untracked-файлов. Это не release baseline и не процент готовности.
- `test-quick` использует `go test ... | grep`, `test-all` также использует `tail/grep` без сохранения статуса `go test`: ложный успешный exit code воспроизведён для test-quick.
- `LABEL AUDIT` в pipeline остаётся комментарием в assembly; LLVM/GPU содержат TODO-fallbacks. Это проверка исходников, не отдельное воспроизведение всех miscompile.
- Рабочее дерево содержит пользовательские untracked-файлы. Showcase сканирует файловую систему, поэтому они влияют на набор тестов. Текущий прогон **не является чистым release baseline**; эти файлы не входят в данный коммит.
- Полный `test-all`, GPU execution, интерактивные demos и сравнение с SDCC в этой сессии не выполнялись.

Августовские **37/39 C Z80 asserts**, **28/30 ABAP assemble**, **59/129 MinZ parse** — исторические измерения, а не сегодняшние результаты. Сентябрьский [аудит](../../reports/2026-09-16-Compiler-State-Audit.md) прямо указывает старую базу `77f8ed19`: его дефекты сначала перепроверять на текущем production-пути.

## Как читать приоритеты

- **P0** — без этого нельзя доверять результату; **P1** — ближайшая практическая ценность; **P2** — развитие после базовых gates; **P3** — отложено до доказанного спроса/выигрыша.
- **Q** — quick win, ориентир до 1–2 рабочих дней; **F** — фундаментальная задача, несколько дней/итераций; **E** — эксперимент с неопределённым исходом. Это оценки масштаба, не сроки обещанной поставки.
- **U5…U1** — ожидаемая полезность: от защиты всех пользователей/разблокировки нескольких треков до узкой опции.
- **V** — проверено сейчас кодом/командой; **H** — есть историческое воспроизведение; **?** — гипотеза, первым шагом нужен repro. Статус относится к проблеме, не к готовности решения.
- Порядок внутри ветки — рекомендуемый порядок работ. Между ветками действуют зависимости в карточках ниже. `[ ]` означает открыто; завершённые изменения перечислены отдельно.

## Дерево работ

```text
MinZ
├── A. Достоверность сборки и тестов — P0, U5
│   ├── [x] A1 [P0 Q U5 V] Сохранить exit code go test в Makefile
│   ├── [x] A2 [P0 Q U5 V] Привести CI к go.mod и реальным build-командам
│   ├── [ ] A3 [P0 F U5 V] Чистый baseline: manifest корпуса, JSON-результаты, причины skips
│   ├── [ ] A4 [P0 F U5 H] Compile → MZA → Z80 execution + dual MIR2/Z80 asserts
│   ├── [ ] A5 [P1 F U4 H] Разнести fast / integration / oracle, ограничить зависания
│   └── [ ] A6 [P1 F U4 V] Пересобрать nightly/security/benchmark на текущих командах
├── B. Production MIR2 → Z80: корректность — P0, U5
│   ├── [ ] B1 [P0 F U5 V] Undefined labels: triage → исправления → fatal diagnostic
│   ├── [ ] B2 [P0 F U5 H] ABI/parallel copies, div/mod, live values через CALL
│   ├── [ ] B3 [P0 F U5 ?] SMC: side effects, recursion/reentrancy, RAM/ROM eligibility
│   ├── [ ] B4 [P1 F U4 H] CFG/block arguments, pointers/structs, IX/IY legality, spills
│   ├── [ ] B5 [P1 Q U4 V] Unsupported op/terminator должен завершаться ошибкой
│   └── [x] B6 [done PR #47] Signed i8/i16 ordering: MIR2 VM ↔ Z80 parity на границах знака
├── C. Frontends и контракт языков — P1, U4
│   ├── [ ] C1 [P1 Q U4 V] Разобрать imports Lanz/Lizp и bit-accessor factcheck
│   ├── [ ] C2 [P1 F U4 H] Pascal records и PL/M PTR: минимальные assemble+run repro
│   ├── [ ] C3 [P1 F U4 H] Nanz/C: поддерживаемая семантика и регрессионный corpus; исправить `u16` arrays и `&&` в MIR2 VM
│   ├── [ ] C4 [P1 Q U3 V] Развести parse / lower / compile / assemble / run по языкам
│   └── [ ] C5 [P2 F U3 H] Frill/Lizp/ABAP/ObjC: по одному сквозному сценарию
├── D. Воспроизводимость и готовый toolchain — P1, U5
│   ├── [ ] D1 [P1 F U5 H] Явные версии/хеши таблиц и правил; исключить скрытый CWD/HOME
│   ├── [ ] D2 [P1 Q U4 V] Одна текущая точка входа в статус и команды сборки
│   ├── [ ] D3 [P1 F U4 H] Headless install/smoke из чистого checkout, pinned dependencies
│   ├── [ ] D4 [P2 F U3 H] Инвентаризация legacy consumers → ADR → архивирование
│   └── [ ] D5 [P1 F U4 V] Новый release workflow: tag build, smoke, dry run, артефакты
├── E. Пользовательские сценарии и runtime — P1/P2, U4
│   ├── [ ] E1 [P1 Q U4 H] Один золотой CP/M пример: исходник → .com → expected output
│   ├── [ ] E2 [P1 F U4 H] IRC/TUI в MZV: mock событий → render/input → transport
│   ├── [ ] E3 [P2 F U3 H] Tetris CP/M и ZX: воспроизводимый input replay/state checks
│   ├── [ ] E4 [P2 F U3 H] SQL/ABAP: показать границу Z80-кода и host bridge
│   └── [ ] E5 [P2 F U3 ?] LSP/debugger: один рабочий edit → error → debug сценарий
├── F. Измеримая скорость и размер — P2, U4
│   ├── [ ] F1 [P1 F U5 H] Benchmark protocol: семантика, bytes, T-states, compile time
│   ├── [ ] F2 [P2 E U4 H] PFCCO on/off, adapter costs, веса рёбер call graph
│   ├── [ ] F3 [P2 E U4 H] Iterator/DJNZ и LUT alignment на реальных горячих циклах
│   └── [ ] F4 [P2 F U3 H] Constant mul/div в IR; частоты срабатывания peephole families
├── G. Исследования: таблицы и oracle — P2/P3, U3
│   ├── [ ] G1 [P2 F U4 V] Физически отделить shared machine/tables от solver
│   ├── [ ] G2 [P2 F U4 H] Aliases и единая cost model до допуска allocation tables
│   ├── [ ] G3 [P2 E U4 H] Converter + hit/miss + независимая проверка на holdout corpus
│   ├── [ ] G4 [P2 F U3 H] Oracle: swap cycles, PFCCO encoding, CFG prepasses, отчёт ошибок
│   ├── [ ] G5 [P2 E U3 H] LIR: честное имя алгоритма, conflict retry, запрет invalid output
│   ├── [ ] G6 [P3 E U2 ?] EXX batch spill / новые tiers: один измеряемый прототип
│   ├── [ ] G7 [P0 Q U5 V] ENRT/Z80T: полная проверка записи, domains, count и limits
│   ├── [ ] G8 [P0 F U5 V] GPU: согласованная пара cost/assignment и host rescore
│   └── [ ] G9 [P0 F U5 V] Stable shape IDs: sparse indexing, worker-independent order
├── H. Отложенное расширение — P3, U1–U2
│   ├── [ ] H1 [P3 E U2 ?] Новые языки/ISA/GPU targets только с владельцем и E2E-сценарием
│   ├── [ ] H2 [P3 F U2 H] Новый TUI/View DSL после устойчивого E2
│   └── [ ] H3 [P3 E U1 ?] Совместный module-wide solver после выигрыша локальных моделей
└── J. Z-machine v3 как сквозной тест зрелости Nanz — P1/P2, U4
    ├── [ ] J1 [P1 Q U4 V] Зафиксировать story, тестовые истории, трассу и эталон
    ├── [x] J2a [P1 Q U4 V] West of House: Nanz core в mzv, полный transcript и CI smoke
    ├── [ ] J2b [P1 F U4 ?] Независимые opcode/state fixtures, больше Z3 и диагностика unsupported
    ├── [x] J2c [P1 Q U4 V] Полный CZECH v0.8 Z3: 349 assertions, 0 failures, 19 print cases достигнуты
    ├── [x] J2e [P1 Q U4 V] mzv: 128K адресация, save/restore, потоки 2–4 и input stream 1 с fixtures
    ├── [ ] J2d [P1 F U4 ?] Полный Z3: экран/status, sound, error boundaries и длинные игровые транскрипты
    ├── [ ] J3 [P2 F U3 ?] Позже: тот же core через MZA + Z80, сравнение трасс/состояния
    ├── [ ] J4 [P2 F U4 V] Spectrum 128K: банкование, память, I/O и формат поставки
    └── [ ] J5 [P2 E U3 ?] Zork I startup/LOOK/команды, затем отдельный demo-артефакт
```

### Трек J: порядок и приёмка

1. **J1 — быстрый фундамент для сравнения.** Пинованный Z-machine v3 story hash и лицензия; маленькие независимые test stories; ожидаемые output, память, стек, PC и ветвления. Эталон запускается отдельно от MinZ. Не использовать старый CP/M Zork в `mze` как доказательство Nanz-компиляции.
2. **J2 — семантика.** [West of House](../../examples/zmachine/zork-west-demo/README.md) уже проходит через Nanz core и `mzv`: полный детерминированный transcript сверяется в CI. Следующий шаг — отдельные opcode/state fixtures и расширение покрытия v3. Go host используется только для загрузки файла и терминального ввода; декодер и интерпретатор написаны на Nanz.
3. **J3 — отложенный Z80-путь.** По текущему приоритету пользователя сначала закрываем J2d и полный проверяемый Z3-suite на `mzv`; к `mz` → `mza` → `mze`/`mzx` вернёмся после этого. Тогда трасса и конечное состояние должны совпасть с J2 и эталоном; wrong-code превращается в минимальный Nanz regression для A4/B/C.
4. **J4 — платформа.** Отдельный 128K memory layout и banked story reader, бюджет ROM/RAM/stack и загрузчик; тест на границе 16K страниц. Текущий `mza` создаёт 48K SNA, а headless `mzx --tap` ещё не устанавливает tape trap: готовность TAP не предполагается.
5. **J5 — тяжёлый сценарий.** Воспроизводимо собранный MIT Zork I v3 проходит старт, `LOOK` и короткий детерминированный transcript на `mzv` и Spectrum; лимиты памяти и времени измерены. Только после этого решать, класть ли отдельный `zvm-zx` demo в релиз. `mzv` остаётся host-инструментом.

J1 можно начать параллельно с текущими P0-задачами; первый работающий J2a уже есть, а J2b и J3 зависят от A3/A4/C3. J4/J5 не блокируют узкий релиз D5. Подробные evidence, зависимость от 128K и состав пакетов — в связанном отчёте.

Первый J1 fixture: [MIT Zork I «West of House» micro-demo](../../examples/zmachine/zork-west-demo/README.md), Z3 story 2 378 байт с исходником и воспроизводимой сборкой. `mzv` исполняет его через Nanz core; независимый opcode/state oracle и trace schema ещё не готовы. Spectrum-интерпретатора пока нет.

**Найдено при J2a (C3), исторический repro:** минимальный `global vals: [u16; 4]` с `vals[1] = 1695` в `mzv` читался как `0:159` вместо `6:159`; `if x >= 32 && x < 127` в функции давал `mir2.VM: cannot resolve symbol "x"`. В Z3 core оба случая тогда обошли байтовым хранением и вложенными `if`; исправления и регрессии указаны ниже. Для headless `mzv` также устранён выход всего процесса на EOF stdin и потеря строк из-за двух конкурирующих читателей; девятикомандный transcript проверяет этот путь.

**Срез C3 от 2026-10-01:** Nanz `&&` и `||` разбираются как отдельные логические операторы и понижаются в MIR2 с коротким замыканием; `TestLogicalShortCircuit` проверяет границы, порядок операций, побочные эффекты и условие цикла. Индексирование массивов теперь наследует тип элемента: `TestU16ArrayRoundTrip` проверяет `u16`-запись/чтение глобального и локальных массивов, динамический индекс и сохранение соседних элементов. Оба исправления подтверждены на MIR2 VM. Проверка `assert in_range(31) == 0 via z80` получила `1`: Z80-исполнение логического CFG требует отдельной диагностики.

**Применение в Z3 core (2026-10-01):** локальные слова и operand stack переведены с пар `[u8; 4096]` на отдельные `[u16; 2048]`. Snapshot по-прежнему сериализует каждые 4096 байт little-endian; MIT save/restore fixture и полный Zork I дали побайтово идентичные save-файлы до/после каждого изменения, а новый core восстановил старый save-файл. Return address arrays остаются байтовыми: PC занимает 17 бит, поэтому одного `u16` недостаточно.

**Callable-срез Nanz (2026-10-01):** [исполняемая проверка и границы](../../reports/2026-10-01-Nanz-Higher-Order-Capabilities-RU.md) разделяют статические лямбды, частичное применение и iterator-захват от пока неподдержанных escaping closures. Восстановлен разбор безаргументной `||` после добавления логического OR.

[Проверка внешнего Z3-корпуса](../../reports/2026-09-30-ZMachine-V3-Test-Corpus-RU.md): CZECH v3 — 10 752 байта и группы opcode с возможностью пропуска; готовая MIT-игра Dark Pit — 27 490 байт. Для J1/J3 сначала маленькие fixtures и CZECH, затем игра как интеграционный тест. CZECH распространяется под собственной разрешительной лицензией, поэтому пока только pinned external input, без копии в репозитории.

**J2c/J2e (2026-09-30):** `czech-smoke.sh` проверяет SHA-256 внешнего Z3 story: 368 tests, 349 passed, 0 failed, 19 print cases. С Frotz сверяется текст всех print cases, кроме отличий в расположении пустых строк. [MIT fixtures](../../examples/zmachine/z3-fixtures/README.md) отдельно проверяют вложенный memory stream, transcript, запись/воспроизведение команд, persistent save/restore с проверкой идентичности story, дополнительные символы ZSCII с UTF-8 выводом и вызов процедуры у границы 128K. Полный MIT Zork I 86 928 байт проходит шесть команд `LOOK → OPEN MAILBOX → TAKE LEAFLET → READ LEAFLET → INVENTORY → QUIT` с совпадением игрового текста с Frotz, а также `save → restore`; Dark Pit 27 490 байт — старт, `look`/`inventory` и `restart`. J2d остаётся открытым: экран/status, sound, ошибки на границах и более длинный gameplay пока не доказаны; UTF-8 ввод дополнительных символов проверен отдельным fixture. Общий `go test -short ./pkg/nanz` остаётся красным на bit accessor, Lanz/Lizp imports и Z80 showcase; `pkg/pipeline`, мета-функции и Z3 smoke зелёные.

## Карточки: первый шаг, зависимости, критерий готовности

| ID | Что сделать и как принять |
|---|---|
| A1 | Убрать маскировку exit code пайпами или явно передавать статус тестового процесса. **Done:** искусственно падающий тест даёт ненулевой код `make test-quick` и `make test-all`, лог доступен полностью. |
| A2 | Использовать Go из go.mod, существующие build targets; убрать/обособить отсутствующий root npm setup, выделить ABAP deps. **Done:** чистый CI доходит до тестов; красные тесты дают красный job. После A1. |
| A3 | Запуск в чистом checkout, версия toolchain/assets, фиксированный manifest tracked-примеров, expected failures по полному пути. **Done:** pass/fail/skip/timeout и общий denominator сохранены в артефакте; untracked не меняет набор; известные дефекты имеют ID и не выдаются за pass. |
| A4 | CLI-default и тесты должны выполнять один pipeline; проверить ABI bootstrap и фактическое число Z80 asserts. **Done:** опорный корпус действительно собирается MZA и исполняется на Z80; wrong-code отрицательный контроль ловится; floors не снижаются. После A3. |
| A5 | Отдельные бюджеты на тест/процесс, отмена дочернего solver, причины пропусков. **Done:** fast tier имеет измеренный бюджет, integration/oracle заканчиваются результатом либо диагностированным timeout; новые production failures блокируют merge. |
| A6 | С нуля собрать плановый полный тест, security scan и benchmark с Go из `go.mod` и явными зависимостями. **Done:** каждый job запускается вручную на чистом checkout, публикует воспроизводимый результат и не маскирует известные failures; включить расписание только после проверки. После A3/A5. |
| B1 | Собрать список undefined labels; проверить inline-asm references/DCE и error intrinsics. **Done:** standalone executable с неразрешённым символом не получает успешный exit; явные внешние символы допустимы лишь в документированном режиме. Known failures допускаются в test manifest, не как молчаливый успех компилятора. После A3. |
| B2 | Малый repro div8(10,3), mod8(13,5), double_sum(3,4) сейчас PASS; расширить ABI/edge cases и cyclic moves именно на PBQP; фиксировать emitted ABI. **Done:** значения и сохранность live registers проверены Z80 execution при разных размещениях аргументов; oracle GCD отдельно в G4. После A4. |
| B3 | Сначала минимальные repro чтения patch slot и рекурсивного call graph. **Done:** оптимизации учитывают эффекты; небезопасные SMC-преобразования отклонены/отключены; тесты покрывают рекурсию, writable code и сохранение значения после patch. После A4. |
| B4 | Начать с screen block-argument mismatch и arena/pointer repro из Open_Bugs_RCA, сверить текущий статус. **Done:** verifier после CFG transforms; MZA+execution для loops/pointers/struct methods; нет invalid IX/IY instructions и spill в запрещённую память. После A4. |
| B5 | Инвентаризировать TODO/default ветки LLVM/GPU. **Done:** непокрытая операция сообщает target/function/op и даёт nonzero; поддержанные примеры продолжают работать. Не приравнивать это к полной поддержке backend. |
| B6 | **Закрыто PR #47:** сравнения `i8/i16` с явным signed `SrcTy` проверены для `< <= > >=` на границах, в ветке и при `bool → u8`, независимым Go oracle, MIR2 VM и Z80 emulator. Старые MIR2 cases без `SrcTy` сохранили прежний unsigned-путь. ABI-aware runner и другие ширины остаются в R2.2/R2.4. [Репорт](../../reports/2026-10-01-R2-Signed-Z80-Compare-RU.md). |
| C1 | Глубокий аудит: Lanz/Lizp CLI imports + Z80 asserts PASS. Разобрать 3 наблюдаемых unit failures: pruning API, imports или неверное ожидание syntax. **Done:** документированное поведение, минимальные regression tests и отдельный сквозной пример; не удерживать неиспользуемые функции только ради старого теста. |
| C2 | Repro `records.pas`, `hello.plm`, `sum_array.plm` на PBQP. **Done:** результат MZA+execution корректен либо неподдерживаемая конструкция получает явную ошибку. После A3; связать с B1/B4. |
| C3 | Приоритизировать arithmetic widths/signedness, arrays/pointers, calls и structs; проверить исторические `import_test.c`, `struct_promote.c`. **Done:** versioned support matrix и dual-run regressions, без декларации «весь C23». После A4/B2. |
| C4 | Для каждого frontend/target показать отдельные стадии и ссылки на команды/fixtures. **Done:** генерация assembly не считается run, skip не считается pass; ObjC/C маршрут обозначен явно. После A3. |
| C5 | Выбрать реальные примеры из fun/examples, включая внешние ABAP dependencies. **Done:** по сценарию есть reproducible command, expected output и проверяемая стадия; остальные возможности обозначены experimental. После C4. |
| D1 | Найти все asset loaders и consumers, выбрать embed для малых данных и manifest для больших. **Done:** одинаковый output из двух cwd без sibling checkouts; asset hash и отключённые/недоступные оптимизации видны; тесты с таблицами и без. |
| D2 | Обновить README/STATUS/snapshot/CLAUDE current-ссылки, исторические цифры пометить датой и revision. **Done:** новичок находит production backend, install и support matrix из README; старое «VIR default» не выглядит текущей рекомендацией. Этот бэклог — первый шаг, не вся задача. |
| D3 | Минимальный compiler+assembler+headless runtime профиль отдельно от GUI/research. **Done:** чистая установка собирает и выполняет E1 без личных путей; dependencies и hashes заданы. После A2/D1/E1. |
| D4 | Найти CLI/library/test consumers старых parser/semantic/codegen/CTIE и QBE round-trip. **Done:** ADR keep/archive/remove и отдельные безопасные миграции; shared LIR/VIR зависимости сохранены. |
| D5 | Сконструировать новый релизный workflow из проверенных `go build` и smoke-команд. **Done:** dry run тега создаёт платформенные артефакты и checksums из чистого checkout, публикация выполняется только после успешных проверок; ни один известный failure не скрыт через `|| true`. После A3/D3. |
| E1 | Выбрать tracked CP/M hello с вычислением, зафиксировать target/команды/вывод. **Done:** один smoke запускается через MZA+MZE из чистого checkout. После A3; discovered backend defects → B. |
| E2 | Взять апрельский handoff, сделать tracked mock для status/log/input/PING/EOF. **Done:** детерминированный сценарий не зависает и рисует ожидаемые состояния; сетевой smoke отдельно и опционально. Без зависимости CI от публичного IRC. После A3. |
| E3 | Сначала подтвердить актуальные Tetris failures, затем input replay с ограничением шагов. **Done:** spawn/move/rotate/line-clear проходят на заявленной платформе; screenshots дополняют проверки состояния. После B4/E1. |
| E4 | Зафиксировать SQL fixture и host ports/SQLite dependency. **Done:** одинаковые запросы/результаты в repeatable demo; документация различает нативное выполнение и услуги хоста. После D3. |
| E5 | Проверить существующие cmd/mzlsp, pkg/lsp, pkg/dap/debugger вместо старого «not started». **Done:** минимальный editor smoke, корректные diagnostics/source locations; масштабировать только после использования. |
| F1 | Семантически равные программы, pinned SDCC/flags, bytes/T-states и latency, все failures в denominator. **Done:** script + raw results + provenance повторяются на чистой установке. После A4/D1. |
| F2 | Ablation PFCCO on/off на leaf/non-leaf, адаптерах и разных профилях call graph. **Done:** нет execution regressions; выигрыш и проигрыш опубликованы по обеим метрикам. После B2/F1. |
| F3 | Измерить iterator back-edge и LUT alignment, затем по одному преобразованию. **Done:** execution equivalence и выигрыш с учётом padding/code size; нет обещанного процента до замера. После B4/F1. |
| F4 | Проверить, какие таблицы доступны production consumers после удаления VIR; переносить правило туда, где константа ещё в IR. **Done:** ненулевые measured hits, корректные flags/carry, ablation. После D1/F1. |
| G1 | Сверить фактические imports с ADR-0043 и вынести shared code отдельным пакетом. **Done:** таблицы/описание ISA не тянут solver; production/tool builds и tests сохраняются. |
| G2 | В полном проверенном 4v-файле 32 211/123 453 feasible assignments имеют physical aliases; GPU H/HL repro принят. Формализовать overlap, legality и модель стоимости Go/GPU/SMT; сохранить mixed-width guard consumer. **Done:** конфликтующие B/BC и H/HL назначения отвергаются; устаревшие таблицы маркированы; generator validation до включения в production. |
| G3 | Конвертер формата после уже внедрённого header rejection; held-out программы и независимая проверка. **Done:** hash/version/counts, hit/miss/reject/fallback counters, корректность Z80 и ограниченная формулировка optimality. После G1/G2/D1/F1. |
| G4 | Отдельный oracle backlog: cyclic moves/GCD, PFCCO formulation или reuse production contracts, одинаковые CFG prepasses. **Done:** sat/unsat/error/timeout различимы, отчёт cost/constraints воспроизводим. Более короткий output сам по себе не доказывает корректность. |
| G5 | Проверить retry/label stubs на текущем LIR; сначала запретить invalid output, затем сравнить heuristic/search. **Done:** противоречие даёт диагностируемый failure/fallback, название соответствует механизму, есть ablation. После G2/A4. |
| G7 | Repro header count=2/body=1 принят loader без ошибки; ENRT с отсутствующими flags/metrics тоже принят. Invalid location IDs и interference bits не отвергаются. **Done:** mismatch/truncation/invalid dimensions дают error, размер и память ограничены до allocation; regression test для Z80T v2 отдельно от уже исправленного ENRT.
| G6 | Один EXX batch-state prototype с call/interrupt constraints. **Done:** реальная pressure workload выигрывает, safety predicates проверяются. После G2/F1; при отсутствии выигрыша остановить эксперимент. |
| H1 | До реализации указать пользователя, maintainer, минимальный test corpus и стоимость поддержки. **Допуск:** базовый correctness milestone закрыт; новый target имеет execute gate. |
| H2 | Сначала получить устойчивый E2 и список повторяющихся пользовательских проблем. **Допуск:** DSL решает подтверждённую проблему, не маскирует backend/runtime failure. |
| H3 | Зафиксировать гипотезу и лимит эксперимента. **Допуск:** G4/F2 дают надёжный baseline и есть cost gap, который нельзя закрыть проще. |

## Ранжированный старт: что делать следующим

Это порядок начала задач; зависимости могут потребовать завершить фундаментальный блок до следующего шага.

| Место | Шаг | Почему сейчас | Тип |
|---|---|---|---|
| 1 | A1 — статус тестовых команд | Очень дешёвая защита от ложного green во всех последующих работах | Q |
| 2 | G7 — Z80T v2 count/limits | Малый воспроизведённый дефект на границе доверия к таблицам | Q |
| 3 | A2 — рабочий CI | Делает результат команды воспроизводимым для каждого изменения | Q |
| 4 | A3 — чистый baseline | Отделяет кодовые дефекты, окружение и личные файлы | F |
| 5 | D2 — согласовать текущие статусы | Убирает повторное выполнение закрытых задач и ошибочный выбор backend | Q |
| 6 | A4 — Z80 execution gates | Ловит именно wrong code, который VM не замечает | F |
| 7 | B1 — labels и invalid output | Превращает неполный executable в видимую ошибку | F |
| 8 | B2 — ABI/arithmetic/calls | Исправляет широкий класс потенциальных production miscompile | F |
| 9 | C1 + B5 — imports и честные ошибки | Небольшие изолированные задачи с быстрым видимым результатом | Q |
| 10 | D1 — assets/provenance | Делает смысловыми сравнения и поставку на другой машине | F |
| 11 | E1 → D3 — CP/M smoke и поставка | Первый компактный, проверенный пользовательский результат | Q → F |
| 12 | B3/B4 + C2/C3 | Закрывает следующие подтверждённые correctness blockers | F |
| 13 | E2 и F1 → F2 | Практический runtime-сценарий и доказательный путь к оптимизации | F → E |


Предлагаемое распределение внимания до correctness milestone: **60% A/B/C, 25% D/E, 15% F/G**. Это рекомендация по фокусу, а не требование параллельно открывать все треки. Одновременно держать одну фундаментальную задачу и один небольшой независимый quick win.

## Вехи и перспективы

1. **M0 — видим реальность:** A1–A3/A5, C4; CI выдаёт достоверный статус, у каждого failure есть категория/ID. Красный baseline допустим как измерение, но не как готовый релиз.
2. **M1 — узкий correctness release:** A4, B1–B4 для поддерживаемого корпуса, C3, D1/D3, E1. Опорный Nanz/C→Z80 корпус без незаявленных failures, установка повторяема. Неподдержанные случаи получают явную ошибку.
3. **M2 — полезные приложения и доказанные выигрыши:** E2, выбранные E3/E4, F1–F3. Выбирать оптимизации по workloads, публиковать bytes/T-states/compile-time и ограничения.
4. **M3 — исследовательский результат:** G2–G4 и PFCCO ablation. Самая близкая тема по существующему коду — PFCCO; самый интересный более дальний эксперимент — повторное использование allocation signatures. Новизна и глобальная оптимальность здесь **не установлены** и требуют отдельной проверки.

Реалистичная перспектива — сильный специализированный toolchain и исследовательская платформа для нерегулярной ISA. Обещать универсальную зрелость всех frontends/targets или дату v1.0 по текущим данным нельзя. Следующее решение о расширении принимать после M1, по фактическому использованию и результатам корпуса.

## Уже сделано — не открывать повторно

- `7f06699b`: QBE immediate operands; `6b6de63f`: зависание string literal с кавычкой; `10f36bbf`: Lizp let-in mangling.
- `eaf96cd9`: carry guard для mul-table; `dfe81d17` и `87a02130`: отключение default VIR, затем удаление из compiler и offline oracle.
- `b45d539f`: enriched header/count validation, Z3 через PATH/override, solver complaints как ошибки. Converter и исправление самой PFCCO formulation этим не закрыты.
- `bd4f98cc`: short guards, test-quick, увеличенный timeout, ratcheting parse floor. Достоверный exit code, полный corpus gate и CI этим не закрыты.
- `f51171a5`: обновлены build-команды и inventory в CLAUDE; другие status-файлы всё ещё требуют D2.

Подробная предыстория: [ремедиация](../Codegen_Remediation_Roadmap.md), [innovation agenda](../Innovation_Agenda.md), [ADR-0043](../adr/0043-vir-demoted-to-offline-oracle.md), [августовский seed](../../contexts/2026-08-21-seed-sprint1-continue.md), [IRC/TUI seed](../../contexts/2026-04-09-seed-irc-tui-runtime.md), [Open Bugs RCA](../Open_Bugs_RCA.md).

Правило обновления: закрывать пункт ссылкой на commit и проверку; историческую находку сначала воспроизводить на указанном backend; менять оценки после первого repro; не переносить failures в skips ради зелёного отчёта.


## Дополнительные треки экосистемы после расширенного аудита

Эти ветки продолжают основное дерево. Работы в соседних репозиториях здесь только запланированы; данный коммит меняет документацию MinZ.

```text
Экосистема MinZ
├── I. PRNG / GPU-графика — gpuforce
│   ├── [ ] I1 [P1 Q U4 V] Зафиксировать local diff, seed/parameters и artifact manifest
│   ├── [ ] I2 [P1 F U4 H] Один fixture → payload → Z80 decoder → bit-exact replay
│   └── [ ] I3 [P2 E U3 H] Joint/cascade search: Pareto quality / total bytes / decode time
├── J. AY/PT3 / audio search — gpuforce
│   ├── [ ] J1 [P0 F U5 V] AudioMixer lifecycle: отмена → join → close, audio test failures
│   ├── [ ] J2 [P1 Q U4 V] Машинная fixture matrix вместо противоречивого 10/12
│   ├── [ ] J3 [P1 F U4 H] First-divergence: disco08 / take5 / Beautiful Agony
│   ├── [ ] J4 [P2 E U3 H] Atlas/search и PSG→WAV после parity/lifecycle gates
│   └── [ ] J5 [P1 Q U4 V] Reg7 не сбрасывает envelope selection reg8/9/10
└── K. Общие артефакты и superoptimization — Z80/6502/архивы
    ├── [ ] K1 [P1 Q U5 V] Реестр producer → format → verifier → consumer
    ├── [ ] K2 [P1 F U5 V] Различать exhaustive / sampled / unverified rules
    ├── [ ] K3 [P2 E U4 H] Одна peephole family в production с hit count и ablation
    ├── [ ] K4 [P2 Q U3 V] Отобрать полезные archive fixtures/stashes по patch-id
    └── [ ] K5 [P0 Q U5 V] Converter: error/timeout/unknown отдельно от infeasible
```

| ID | Первый шаг и критерий готовности |
|---|---|
| I1 | Снять provenance для изменённого prng_budget_search и выбранного результата. **Done:** revision+diff hash, parameters, seed, input/output hashes; восстановление не зависит от личного binary. |
| I2 | Выбрать одну небольшую картинку и существующий decoder. **Done:** CPU и target replay дают одинаковый bitmap; измерены payload+decoder bytes и T-states. После I1; не требует завершения G. |
| I3 | Сравнить greedy/joint/cascade при одинаковом бюджете. **Done:** raw results и Pareto-таблица на нескольких fixtures; отсутствие выигрыша тоже фиксируется. После I2. |
| J1 | Воспроизведены audio failures и send-on-closed-channel panic. **Done:** тесты проходят, shutdown повторяем, race check для lifecycle/mixBuffer; Stop не закрывает канал раньше завершения отправителей. Независимо от compiler B. |
| J2 | В документе 12 fixtures, 3 non-perfect, но headline 10/12. **Done:** generated matrix с oracle/version/tolerance/frame denominator; no-test-files не считать parity validation. |
| J3 | Сначала обновить исторические repro. **Done:** исправления подтверждены first-divergent-frame regression, новая matrix сохранена; исключения объяснены. После J1/J2. |
| J4 | Разделить player fidelity и качество синтезированного звука. **Done:** reproducible PSG/WAV fixture и измеримый search objective. После J3. |
| K1 | Начать с JSON-инвентаря аудита; проверить zero-byte/partial артефакты, pinned Go launcher. **Done:** каждый используемый asset имеет hash, producer revision, schema, domain, consumer и способ восстановления; большие файлы не добавляются в Git автоматически. |
| K2 | verifier.go использует reduced sweep при >=3 extra regs/SP. **Done:** verification mode и assumptions записаны на правило; sampled не называется proof; независимая emulator validation для допуска в compiler. После K1. |
| K3 | Взять подход family normalization из 6502 и одну частую форму из MinZ output. **Done:** preconditions/flags/clobbers проверены, measured hits и bytes/T-states improvement без execution regressions. После K2/F1. |
| K4 | Начать с archive README и существующей research/pfcco-paper-v2. **Done:** маленький список уникальных fixtures/текстов с provenance; уже присутствующие LIR hardening patches повторно не переносить. |

Внутри экосистемы **J1 — P0 для audio**, а **G7 — P0 для table consumers**; это не утверждение, что данные пути используются production CLI. Быстрый параллельный по смыслу результат: I1/K1/J2. Фундамент: J1/I2/K2. Новые GPU-прогоны — после фиксации модели и критериев приёмки.


## Корректировки после глубокого аудита: обязательные gates GPU-трека

| ID | Новое свидетельство | Критерий готовности |
|---|---|---|
| G8 | GPU вернул 12/80 несогласованных cost/assignment pairs на маленьких задачах с известным optimum; результат scheduling-dependent | Pair reduction либо второй witness pass, host feasibility/cost rescore, deterministic tie-break; regression stress без mismatch |
| G9 | При одном worker 160/162 позиций feed не соответствуют consumer index; реальный dense record 1549 содержит 6v вместо ожидаемых 3v | Shape ID проходит generator→server→converter→consumer; format хранит ordering/filter/domain version; sparse subset не используется как full positional table |
| K5 | Actual converter принял server parse error как `0xFF` infeasible | Tagged result type, failed/partial job не публикуется как полная таблица; negative controls проверяют status propagation |
| J5 | Перестановка reg7/reg8 writes меняет 4390/4410 sync samples; repeat того же порядка совпадает | Сохранение независимых register fields, order-invariance tests, deterministic synchronous render |

Последовательность для новых GPU-артефактов: **K5/G7 → G8 → G9 → G2 → независимая проверка → G3 integration/coverage**. Aliases и schema проектировать совместно; полную перегенерацию начинать только после всех model/identity gates. Эти P0 относятся к experimental artifact pipeline; ordinary PBQP compiler не зависит от VIR.

Положительные результаты глубокого прохода: 16 focused MIR2 tests PASS; обе cross-language CLI programs PASS с Z80 asserts; Che cascade compile+assemble PASS; sync AY выдаёт ненулевой повторяемый звук. Поэтому C1 — triage контракта unit tests, I2 — доведение существующего decoder до bit-exact replay, J1 — отдельный async lifecycle, а не переписывание всей подсистемы.
