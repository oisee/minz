# Grace / PGO: текущее состояние

11 октября 2026. Рабочая ветка — `feat/grace-hir-vm`.
«Production path» в исследовательских отчётах означает основной путь этой
ветки; это не утверждение, что изменения вошли в GitHub main.

## Новый этап: единый backend без LIR

Проверенная сводка на `cad80340`; результаты прежних сборок ниже сохраняют
свой исходный контекст. Общая цель из [плана](../reports/2026-10-11-017-unified-backend-roadmap.md)
ещё активна: корректность заявленного корпуса, проверенные контракты и таблицы,
выигрыш Grace на разных программах и заранее отложенных нагрузках.

- **LIR полностью удалён из живого компилятора** (`47127a27`). Shared rewrite
  engines, VIR и native оракулы сохранены. Архив 53 research-файлов проверен
  по SHA256: [пакет удаления](grace-lir-final-removal/README.md).
- **193 из 194 текущих showcase собираются** в свежем прогоне на `ed4c1d56`
  (193 PASS, один SKIP; исходные 193 пути плюс новый CP/M demo); отдельный opt-in запуск пропуска — явный FAIL. Один host ZVM
  получает явный отказ: его story[131072] превышает 16-битное адресное пространство.
  Поддержка native u32 memory/caller/spill уже исправлена и проверена отдельно;
  это не делает host interpreter подходящим для обычного Z80.
  Сборка корпуса не доказывает правильность исполнения. Новый
  [актуальный каталог](grace-integration-077/catalog-33-vm-native.json) фиксирует все
  194 точных пути: 33 имеют ограниченное independent VM/native покрытие. Новый CP/M demo
  имеет отдельный native CALL5-hook proof. Все прочие пути сохраняют явные
  pending/ASM/runtime/host ограничения.
- **Pascal bubble sort: исправлен реальный native miscompile** (`ff11ba69`).
  Typed NOT теперь читает и записывает назначенные регистры/spill, bool использует
  XOR 1. 744 прямых VM/native случая и четыре реальные нагрузки прошли;
  сортировка проверяет все десять байт массива, Arena — состояния четырёх арен.
- **Массивы bool исправлены** (`ee21cae8`): каждый элемент занимает адресуемый
  байт. Popcount/FSM/Lizp crypto и Pascal sieve прошли независимые VM/native
  модели; sieve проверяет все 101 флаг и COUNT=45. 18 регрессий потребляют
  живой A после NOT. [Пакет](grace-showcase-algorithm-oracles.md).
- **Self-Lanz: второй строгий prospective heldout** (`1ed22cb0`). Три training
  и шесть disjoint heldout форм зафиксированы до измерения. Speed выбрал append-only;
  тот же BIN на heldout: 2141572 → 2093170 T, 2364 → 2344 B.
  Независимые Go/Python проверяют узлы, строковую арену и порядок доступов памяти.
  [Пакет](grace-parser-prospective-heldout/README.md).
- **HashMap: выбор на training, проверка того же BIN на heldout**. Неизменённый
  исходник; 15 training и 48 отложенных вызовов зафиксированы до выбора.
  Speed/Size выбрали оба ordered-byte helper: 255642→236938 T на heldout,
  981 → 962 B. Независимые Go/Python проверяют результаты, 82 байта состояния и
  доступы памяти каждого вызова. [Доказательства](grace-hashmap-heldout/REPORT.md).
- Исправлены signed Sub/Cmp miscompile и три Lizp ZX примера. Для последних
  все 6912 байта экрана сверены на VM и собранном Z80; ULA/железо не проверялись.
  [Signed](grace-signed-sub-cmp/REPORT.md) · [Lizp](grace-lizp-zx-screen.md).
- Native u16×10/12 получил конкретную пользу: 1079 → 96 T, 32 → 9 B при результате HL;
  это замер операции, не всей программы. [Замеры](grace-mul16-10-12/REPORT.md).
- **PFCCO имеет native измерение с реальными вызовами** (`cb587fe4`):
  weighted packet workload, 17 boundary/reversed/full-width пар, первый результат
  жив через второй CALL. Off: 8857 T / 52 B; greedy и PBQP: 7888 T / 52 B.
  Это конкретная нагрузка, не доказательство глобальной оптимальности.
  `ContractOpt=false` теперь действительно отключает переписывание контрактов.
- **Проверенные локальные таблицы** связывают назначения с versioned ключом,
  descriptor, шаблонами, clobbers и стоимостью реальных moves. Проверка допуска
  и native fixtures прошли (`6be9d4d4`). Подключены к обычному CLI (`5652212c`) через
  `--Osize --checked-local-table`: add5 проходит все 256 native входов,
  8 → 6 B и 56 → 49 T. Офлайн-генерация через Z3 теперь доступна в CLI
  (`74f4864a`). На существующем weighted: 36 → 12 B; с caller harness
  и сохранениями 238 → 142 T, все 256 входов VM/native PASS. ABI/return/clobbers входят в проверку, inadmissible
  запись сохраняет PBQP. Пока только pure leaf single-block функции;
  CFG/островные назначения остаются впереди.
- **Третье семейство имеет prospective heldout** (`131b32f8`): PFCCO packet,
  5 новых training и 12 heldout pairs. Speed выбрал ContractOpt: 6252 → 5568 T
  на heldout при 52 B. Size сохраняет original. Tail-recursion эксперимент
  отверг Grace как более медленный и большой; отрицательные артефакты сохранены.
- **Bool-поля struct исправлены** (`1e3b1bdd`): storage и parser offsets
  отдельно округляются к byte; full MIR2/Nanz/HIR и native fields+guard PASS.
- **Таблицы v3** (`a9f21893`) отвергают непривязанные inputs и hard classes,
  исправляют IX/IY costs и ограничивают host solver deadline. API measured modes
  проходит тесты; аудит поздней мутации завершён (`919b54c2`): после final sentinel
  перепроверяются full HIR, семантические MIR metadata, allocation и assembly
  всех четырёх кандидатов.
- **Оригинальный self_tokenizer проверен** (`c3782f21`): 30 sequential inputs,
  полные 8192/4096/4096-byte memory regions и 47 globals совпадают с независимой
  моделью VM/native. Main и вывод не покрыты. Стенд корректно допускает reachable
  EQU aliases; distinct unreachable code по-прежнему отвергается.
- **Таблицы v4** (`c185aebb`) проверяют типы/arity/physical kind/return ABI
  и мутацию функции после подготовки. Два native контрпримера отвергаются;
  weighted benefit сохраняется. Calls/CFG/islands остаются вне допуска.
- **CLI измеренного выбора режимов** (`eaf8c8b7`):
  `--hir-profile-compiler-modes --hir-profile-auto DIR` использует controlled
  workload, сохраняет imports, все кандидаты и точный выбранный ASM/BIN.
  Speed выбирает минимум T; `--Osize` задаёт отдельную цель Size.
  [Воспроизведение](grace-compiler-modes-cli/REPORT.md).
- **Полный main SelfLanz** (`a1b18f69`, `f1b4bd4c`): исправлен CString для
  чтения до NUL согласно Language Book v7 §18.1. Собранный Z80 и явный CP/M
  BDOS-адаптер прошли независимые проверки 135 байтов вывода, всех globals,
  трёх областей памяти, стека и неизменности кода. Это запуск в эмуляторе,
  не на физическом железе. Исторические prospective эксперименты сохраняют
  неизменяемый исходник и прежние артефакты.
  [Доказательства](grace-original-lanz-main-native/README.md).
- **CLI сохраняет выбранный результат** (`67700e26`, `094ffa44`): compiler-mode
  каталог публикуется целиком атомарно без замены существующего каталога;
  внешний ASM/BIN заменяется атомарно отдельно. Все три memory-host команды
  теперь записывают настоящий BIN при `-o .bin`, ASM при `-o .a80`.
  Реальный collect/replay парсера и независимый аудит всех кандидатов прошли.
  [CLI-пакет](grace-compiler-mode-artifacts/REPORT.md) ·
  [Исправление BIN](grace-memory-host-selected-output/REPORT.md).
- **Ограниченные доказательства allocator** (`8effe44b`): 600 малых PFCCO
  forest-задач сопоставлены с независимым полным перебором; PBQP проверен
  на 36 допустимых назначениях с native стоимостью. Измерения включают
  пересылки и сохранения. Это не глобальная оптимальность любых CFG.
  [Границы доказательства](grace-branch-allocation-proof/README.md).
- **Полный pipeline на `cb587fe4` завершился FAIL за 2618,872 с**.
  Старые эталоны parser и настройка ContractOpt исправлены позднее.
  Условные invariant-кандидаты возвращают 257/513 вместо 1; ошибка
  исправлена в `cc502270`; все падавшие группы повторно прошли. Новый полный
  прогон на `cc502270` завершился FAIL за 1649,277 с: пересечение hoist
  исправлено в тесте, historical policy-search fixtures исправлены в `22e426de`; три полных lifecycle группы прошли за 419,343 с.
  Полный CLI `c784dc4a` прошёл за 374,535 с. Полный pipeline PASS
  не заявляется. [Сохранённый лог](grace-integration-046/pipeline-cb587-full.txt).

Дополнительный аудит byte packing нашёл native контрпример с неявными
флагами OR. Исправление `81e0d8b6` проверяет MIR-условия через копии и
параметры блоков; независимые native регрессии и 2560 случаев с вызовами
прошли. FAT12 выигрыш 98 → 78 T, 21 → 16 B сохранён.
[Граница флагов](grace-byte-pack-state/README.md).

Два дополнительных пути сохранения A при исчерпании регистров также
затирали D. В `0041d9e4` они используют named spill; native регрессия
проверяет сохранность DE и A. Полный MIR2 после изменения прошёл.
[Доказательства](grace-accumulator-exhaustion/README.md).

Полный main исходного self_tokenizer (`078cca62`) проверен на собранном Z80
и с явным CP/M BDOS2-адаптером: 19 токенов, 91 interned byte, 146 байтов
вывода, все 47 globals, 8192/4096/4096-byte RAM, стек и неизменность кода.
Исходник не изменён. Это эмуляция, не запуск на физической ОС.
[Пакет](grace-original-tokenizer-main/README.md).

CLI fixed-winner replay (`cb9e8ba6`, `e43459a6`) восстанавливает только
записанный режим, требует exact ASM/MIR/BIN, воспроизводит его полный training
запуск и затем выполняет тот же BIN на другой workload. 12 frozen PFCCO calls
прошли за 5568 T при 52 B. Остальные кандидаты повторно не измеряются;
оптимальность относительно их записей не является независимым доказательством.
[CLI workflow](grace-compiler-mode-replay/REPORT.md) ·
[Проверка training-записи](grace-compiler-mode-replay-trust/REPORT.md).

Умножение байта (`c6e79f72`) сохраняет живые scratch-пары даже при полном
давлении регистров: 16384 native случая плюс ordinary-source проверки прошли.
Стоимость необходимых PUSH/POP учитывается; ускорение этого случая не заявляется.
[Регрессия](grace-constant-mul-scratch/README.md).

Оба исходных entry `03_filter_map_chain.nanz` (`81df76ed`) прошли 84 raw
VM/native вызова с независимым ordered callback и memory oracle. Runtime process
задан явно; исходник не менялся. [Пакет](grace-original-filter-map/README.md).

MIR Grace теперь одинаково включается через CompileHIRSteps и
CompileHIRWithOptions (`5226309a`): на реальном intern workload совпадают BIN,
независимые результаты/память/traffic и Size policy.

Изменения находятся в `feat/grace-hir-vm`, не объявлены merged в main.
Новые отчёты доступны в репозитории и на `/mnt/safe/minz/grace/2026-10-11/`;
Sites 62 содержит обновлённую сводку и новые архивные пакеты; последующие
коммиты сохраняют отдельный контекст в репозитории и на шаре.


Оригинальный tokenizer теперь также собирается обычным CLI через `--runtime cpm`
(`aa0e4ccf`, `60de5579`): COM 5815 B, 226178 T, 146 байтов вывода;
весь RAM, globals/арены, SP, stack guard и неизменный код проверены.
Это CALL5-hook эмуляция, не настоящая CP/M OS.
[Публичный runtime](grace-cpm-tokenizer-runtime.md).

Обычный opt-in PFCCO PBQP улучшен для dead byte literal RHS (`2ed9d4bb`):
Nanz 84327 → 82283 T, 60 → 58 B; independent native назначения и
отрицательные controls сохранены. Default greedy не менялся.
[Модель и пределы](../reports/2026-10-11-066-pfcco-literal-accumulator-demand.md).

Полный CLI `89fff011` после cleanup: PASS за 383,376 с. Он предшествует
новым tokenizer/PFCCO изменениям. Full pipeline `22e426de` выполняется.


Последний полный core gate `a3cc555f`: HIR/MIR2/Nanz/VIR/CLI PASS; CLI 384,168 с.
Это контекст после runtime, typed_print и callable signature fixes; следующий
test-only coverage не меняет production. [Полный лог](grace-integration-077/core-a3cc555-full.txt).

Рекурсивный selfLanz теперь собирается обычным CLI (`ecda27e1`, `5f5c7c52`,
`c23ba0cc`): COM 4727 B, 983310 T, 17 AST nodes/135 stdout bytes.
Guard проверяется по реальному CFG-пути; forced recursive shared spill и
controlled overflow проходят. Публичный MZE вывод совпал побайтно.
Это BDOS hook, не настоящая CP/M OS. [Runtime](grace-cpm-recursive-runtime.md).

Исправлены field scratch и struct pointer ABI (`42817543`), callable return
signature (`4ba526a3`) и live accumulator scratch relocation (`ee5621ac`).
MultiCall formal integer widening сохранён (`a3cc555f`).
Typed print проверяет настоящий text.print import и persistent buffer;
function-pointer consumer проверен с measured table: 34 → 31 B,
apply(triple,x) 123 → 111 T. Compile/generation/miss overhead сохранён,
не выдаётся за нулевую стоимость. [Интеграция](grace-integration-074/README.md).

## Что уже работает

| Возможность | Состояние | Как включается |
|---|---|---|
| Обычная Nanz → HIR → MIR2 → ASM компиляция | Основной путь ветки | Обычный CLI |
| Структурные Grace-преобразования HIR | Эксперимент, с проверками | `--hir-grace` |
| MIR Grace | Отдельный режим | `--grace` |
| Unsigned16 PreserveForSpeed | Явный opt-in; default Legacy сохранён | Типизированная политика / специальные CLI |
| Конечный поиск policy × arena/append | 8 реальных Z80-кандидатов; полный CLI gate прошёл | `--memory-host-arena-append-search DIR` |
| Чистая цель Speed | Минимум реальных тактов, без штрафа за размер | Speed в специальном поиске |
| Цель Size | Минимум размера BIN | `--Osize` в специальном поиске |
| Общий invariant address hoist | Ограниченные pure 16-bit DAG, single-latch loop без calls/ASM/ports; interning −1,63% T за +8 B | `--grace`, выключен при `--Osize` |
| Публичный CP/M runtime | Оригинальные tokenizer и recursive selfLanz COM прошли native full-state и публичный MZE | `--runtime cpm -t cpm`, для recursion `--runtime-stack 1024` |
| Self-Lanz: полный CP/M main | 17 AST nodes, 135 stdout bytes, все RAM/SP/code; guarded runtime overflow проверен | Обычная CLI `.com`, MZE BDOS эмуляция |

## Свежий полный парсер: collect/replay

Все четыре запуска прошли в контексте CLI 9ec48, исходник и 30-call
workload неизменны. Speed: policy1/mask3 — 1 233 170 T / 2490 B.
Size: policy0/mask2 — 1 236 250 T / 2368 B. Legacy original:
1 291 565 T / 2388 B. Независимый source/state/profile/proof аудит
подтверждает все четыре каталога; второе native-ревью также прошло.
Все 32 ASM пересобраны в точные сохранённые BIN.
1080 исполненных наблюдений; 1200 сохранённых записей содержат
120 скопированных записей входных ассоциаций.

## Результаты полного парсера в сохранённом прежнем контексте

Это исторические измерения до исправлений строк и сохранений регистров.
Свежие замеры приведены выше; исходные артефакты и контекст сохранены.

Неизменённый `self_lanz_parser.nanz`, одна обучающая нагрузка:
- Original Legacy: 1 135 370 T, 2 000 B.
- Выбранный Speed: 1 076 715 T, 2 100 B, −5,17% тактов.
- Выбранный Size: 1 079 795 T, 1 978 B, −22 байта.

Это реальные исполнения собранного Z80 в эмуляторе. Замеров на физическом
железе нет. Полный CLI gate прошёл, оба goals исполнены отдельно, два
независимых аудита проверили новый production контекст.

Заранее выбранные варианты проверены на четырёх известных входах без нового
поиска: native export и два независимых аудита 200 вызовов прошли. Это
ретроспективная проверка переноса на этом парсере, не слепой unseen corpus.

## Исправления, которые нужно отделить для ревью в main

Это подтверждённые примеры из истории ветки; полный dependency inventory
подготовлен в `grace-backend-fix-extraction/`; отдельные PR ещё предстоят.

| Commit | Что исправлено |
|---|---|
| `4034461d` | Эффекты callback fold, стадии map/filter и сохранение живого A при store |
| `76ceefc9` | Range iterator stages, рёбра DJNZ при удалении параметров, живой аккумулятор |
| `dbac514b` | Приведение локальных integer bindings к объявленной ширине и shadowing |
| `d6e3a8d3` | Расширение и сохранение канонических boolean predicates |
| `9533b4ad` | Сохранение живых HL/H/L при прямой записи константы в global |

Полный inventory содержит семь групп correctness-фиксов с точными коммитами,
регрессиями и зависимостями для будущего переноса в main.
Нельзя считать известное падение безвредным лишь потому, что оно старое.

## Исторический gate до нового этапа

Сверка после `a4ac51bd` имела26 падений. В новом этапе import search paths,
ошибки HIR/frontends и native miscompile исправлены; актуальный статус корпуса
указан в начале страницы. Исходный отчёт не переписывается как свежий прогон.

Строковые 15 actual Z80 случаев проходят через оба пути компиляции;
corrected legacy run подтверждён двумя независимыми аудитами, включая
пересборку BIN. Поддержка проверки сохранений spill values на стеке
интегрирована в `a02a3cca`: оба коротких CLI collect/replay прошли,
независимые аудиты проверили 60 наблюдений и выбранные ASM/BIN.
Полный CLI последней decimal-сборки прошёл: 306.414 s, exit 0.
Полный pipeline завершился за 2560.907 s, exit 1: ровно три
устаревших T/BIN эталона, других падений нет. Полный прогон сопоставлен
с уже прошедшими тремя текущими тестами; буквальный package exit 0
не заявляется. Лог, inventory 269 тестов и точный diff сохранены
в `grace-certified-parent-gates/`.
Предыдущий полный pipeline завершился с тремя устаревшими T/BIN эталонами.
В `71b75390` эти три эталона обновлены после отдельных реальных замеров:
все три теста прошли, два независимых аудита подтвердили сохранённое
поведение и BIN. Полный прогон был собран до обновления эталонов; production-код
с того checkpoint не менялся.

Свежий `sum_array` (`b8054d37`) прошёл на VM и Z80 для длин
0/1/5/16/64/255: 18 original/direct/selected измерений. На этой нагрузке
Speed выбирает pointer-walk: 48 495 → 13 879 T, полный бинарь 195 → 250 B.
Size сохраняет исходный вариант. Исходник, профиль, решения, ASM/BIN,
MIR, результаты и точный executable сохранены в
`grace-sum-decimal-cost-refresh/`; прежние 1022-case артефакты сохранены
со своим контекстом и не объявляются новым прогоном.

## Следующие шаги

1. Проверять checked-local таблицы на дополнительных реальных случаях и
   измерять стоимость генерации/компиляции; calls/CFG/islands пока вне допуска.
2. Расширять независимое исполнение корпуса; для host ZVM и Agon определить
   требуемый runtime/target, сохраняя явные ограничения вместо фиктивного PASS.
3. Расширять измеренный invariant address hoist и автоматический выбор
   полезных кандидатов. Memory fusion отложена: порядок partial trap и
   liveness нельзя менять без явного доказательства.
4. Дождаться terminal полного pipeline, исправить обнаруженные проблемы и
   проверить соответствие всей цели по актуальным исходникам.
5. Обновить приватную сводную страницу сайта после следующего gate;
   сохранять source/profile/ASM/BIN/замеры на шаре.

Чистый Speed сохраняется. Парето-фронт и лимит байтов могут стать
дополнительными режимами. Совпадение аддитивных эффектов на одной нагрузке
не разрешает исключать комбинированные замеры произвольных правил.

## Где смотреть

- [CLI и точные результаты](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-search-cli.html).
- [Цепочка Self-Lanz и исходник](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/self-lanz-chain.html).
- [Полный локальный пакет CLI](grace-memory-host-search-cli-artifacts/manifest.json).
- [Ночная сводка](../reports/2026-10-11-011-overnight-summary.md).

Последняя подтверждённая публикация — Sites 62, source `f277b16b`, compiler snapshot `89fff011`. Новые runtime, main, PFCCO и исправления сохранены в архивных пакетах; policy-fixture `22e426de` следует после snapshot. Свежие sum_array, full30
и CP/M main собраны на странице current-state.html. Перенос фиксированных
вариантов опубликован; CLI gate, terminal pipeline и обновление трёх эталонов опубликованы
с исходными exit-кодами и независимым итоговым аудитом. Все внешние страницы и
артефакты сохранены локально с SHA256; исходные исполняемые файлы
восстанавливаются из опубликованного общего архива без потерь.
