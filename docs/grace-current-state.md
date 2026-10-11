# Grace / PGO: текущее состояние

11 октября 2026. Рабочая ветка — `feat/grace-hir-vm`.
«Production path» в исследовательских отчётах означает основной путь этой
ветки; это не утверждение, что изменения вошли в GitHub main.

## Сводка последнего проверенного состояния

Последние code checkpoints: caller-owned struct ABI и owning locals `bbe17da0`,
несколько результатов `161e47b2`, nested value copies `d0fc2059`, точное
клонирование метаданных `cfaa7777`. Подробный [отчёт ABI](../reports/2026-10-11-083-caller-owned-struct-abi.md)
сохраняет выполненные проверки и ограничения. Общая цель не закрыта.

Исправлен новый Grace miscompile `46ced4a1`: BrIf с одинаковым target
сохраняется, если аргументы рёбер различаются. Исходный max_chain снова
возвращает219 вместо36; before/after native proof и ordered iterator
проверки сохранены в [пакете185](grace-integration-185/README.md).
Долгий старый pipeline дополнительно сообщил устаревшие sum cost-formula
ожидания; независимое измерение полного домена в работе.

| Требование | Что доказано | Что ещё не доказано |
|---|---|---|
| Заявленный корпус | Все 194 пути имеют явный статус; 73 independent/bounded VM/Z80 paths | 64 ждут независимого оракула, остальные имеют отдельные ASM/runtime/host ограничения; bounded entry не означает всё приложение |
| PFCCO/PBQP и таблицы | Реальные назначения, moves/saves и native польза на packet/weighted; bounded solver audit | Произвольные calls/CFG/islands и глобальная оптимальность |
| Автоматический выбор и heldout | Self-Lanz, HashMap и Pascal SORT: training-selected BIN проверен на заранее отложенных входах; packet даёт отдельный PFCCO выигрыш | Остальные runtime/corpus ограничения сохраняются; это не универсальный выигрыш на всех программах |
| Полное удаление LIR и перенос пользы | Пакет/consumers удалены; shared engines/native тесты сохранены; word ×10/12, byte fusion и K11 перенесены с native пользой | Полнота общих physical constraints/pattern choices требует предметного аудита |
| Воспроизводимость | Исходники, workloads, решения, ASM/BIN и логи на `/mnt/safe/minz/grace/2026-10-11/`; v67 сайт: 9 860 файлов целиком проверены по SHA | Новые ABI и purity изменения требуют полного gate на новом checkpoint |

Последний root full core `0f0953d9` — HIR/MIR2/Nanz/VIR **PASS**;
full CLI `568b7d08` — **PASS 384,240 с**. Оба предшествуют struct-buffer ABI
([core163](grace-integration-163/README.md), [CLI164](grace-integration-164/README.md)).
Агентский post-clone core прошёл на своём точном checkpoint; его лог сохранён в ABI отчёте.

Новый ABI проходит 180 struct native cases, 256 typed indirect multi-result inputs,
cross-result overlap/wrap и исходный Widemath expanded workload **PASS 68,401 с**
([пакет170](grace-integration-170/README.md)). Четыре прежних frozen heldout
семейства после ABI прошли **PASS 33,374 с** ([пакет168](grace-integration-168/README.md)).
Полный CLI `44d8952b` завершился с кодом 1: три теста ожидают прежний отказ
или предупреждение о косвенных вызовах ([окончательный журнал175](grace-integration-175/README.md)).
Ожидания обновлены после независимого Z80 proof точного исходника; root focused
PASS 0,165 с. Полный CLI `2612a266` завершился PASS, exit 0 ([пакет181](grace-integration-181/README.md)).
Полный MIR2 `e13e04a3` после OpCallCond также PASS ([пакет180](grace-integration-180/README.md)); checkpoints различаются.

Полный pipeline `cfaa7777` выполняется с заранее сохранённым исполняемым
файлом ([пакет169](grace-integration-169/README.md)). Уже сообщил два несовпадения
исторических фиксированных T/SHA в parser baseline и loop association; общий
результат ещё неизвестен. Причина установлена: исправление unsigned extension добавило 27 байт и 27 864 T;
независимый оракул всех 30 входов сохранён, baseline и profile дают идентичные
ASM/BIN/состояния/такты ([отчёт84](../reports/2026-10-11-084-parser-memory-host-reference.md)).
Эталоны обновлены в `5fe88510`; root совместный replay PASS 6,344 с.
Эти live-прогоны не считаются PASS.

Исторический полный pipeline `a7954042` — **PASS 3705,100 с**, до новых ABI изменений.
Последний compile/assemble корпус на `30c2efda`: **193 PASS / 1 capacity SKIP**, 9,358 с ([пакет172](grace-integration-172/README.md));
исполнение всех 194 программ этим не доказано. Текущий каталог —
[пакет189](grace-integration-189/catalog.json). Эти проверки нельзя объединять в один current fullsuite PASS.

Исходные Lizp tokenizer whole-main, R5RS13entry и tuple3module имеют новые независимые
bounded proofs ([пакет179](grace-integration-179/README.md)). VM `OpCallCond` исправлен
в `04122fb2`; source tokenizer сохранён. Ограничения доменов приведены в отчётах.

Чистый Speed сохранён без штрафа за байты; Size — отдельная цель.
Физическое железо остаётся следующим уровнем.

## Новый этап: единый backend без LIR

Сводка дополнена на `29575221`; результаты прежних сборок ниже сохраняют
свой исходный контекст. Общая цель из [плана](../reports/2026-10-11-017-unified-backend-roadmap.md)
ещё активна: корректность заявленного корпуса, проверенные контракты и таблицы,
выигрыш Grace на разных программах и заранее отложенных нагрузках.

- **LIR полностью удалён из живого компилятора** (`47127a27`). Shared rewrite
  engines, VIR и native оракулы сохранены. Архив 53 research-файлов проверен
  по SHA256: [пакет удаления](grace-lir-final-removal/README.md).
- **193 из 194 текущих showcase собираются** в свежем прогоне на `59db4b46`
  (193 PASS, один SKIP; исходные 193 пути плюс новый CP/M demo); отдельный opt-in запуск пропуска — явный FAIL. Один host ZVM
  получает явный отказ: его story[131072] превышает 16-битное адресное пространство.
  Поддержка native u32 memory/caller/spill уже исправлена и проверена отдельно;
  это не делает host interpreter подходящим для обычного Z80.
  Сборка корпуса не доказывает правильность исполнения. Новый
  [актуальный каталог](grace-integration-124/catalog-50-vm-native.json) фиксирует все
  194 точных пути: 50 имеют ограниченное independent VM/native покрытие; новый fib_iter, ранее пропущенная ссылка18tail и исходные local struct вызовы подтверждены отдельно. Новый CP/M demo
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
новым tokenizer/PFCCO изменениям. Full pipeline `22e426de` terminal PASS за 2805,380 с.
[Полный лог и границы контекста](grace-integration-081/README.md).


Последний полный core gate `a3cc555f`: HIR/MIR2/Nanz/VIR/CLI PASS; CLI 384,168 с.
Это контекст после runtime, typed_print и callable signature fixes; следующий
test-only coverage не меняет production. [Полный лог](grace-integration-077/core-a3cc555-full.txt).

Рекурсивный selfLanz теперь собирается обычным CLI (`ecda27e1`, `5f5c7c52`,
`c23ba0cc`, `7fe6cf8d`): после объединения guards COM 3968 B, 415614 T,
17 AST nodes/135 stdout bytes. В одинаковом guarded1024 режиме прежние
4727 B/983310 T сокращены на 759 B/57,7%; tokenizer 8285 → 7397 B,
869959 → 408919 T. Новый root CLI COM совпал с API и прошёл независимый
native full-state оракул. На малом стеке возможен более ранний controlled
overflow; минимальный успешный вход checker теперь требует 38 B.
[Свежая интеграция и каталог](grace-integration-080/README.md).
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

Полный свежий core `9b1637ef`: HIR/MIR2/Nanz/VIR/CLI PASS; CLI395,601с.
Настоящий four_ptrs HL/DE/BC/IX ABI прошёл1792cases; native harness теперь
принимает IX/IY. PFCCO PBQP на оригинальном arena allocator уменьшает
полный BIN1193→1184B; arena_split220→190T с100boundarypairs каждой стратегии,
проверкой destination guards/code/literals/SP. Это польза существующего
opt-in PFCCO, не новый Grace prospective search. [Интеграция082](grace-integration-082/README.md).

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

Свежий refresh `3e143f9f`: parser/HashMap/PFCCO frozen heldout PASS;
HashMap теперь258962→237948T/990→970B на этом compiler контексте.
Явный Z3 weighted/caller/indirecttable gate PASS, поддерживаемые пакеты
compile-only без LIR PASS. Это повторная проверка прежних входов,
не новый unseen experiment. [Пакет083](grace-integration-083/README.md).
Полный pipeline3e143f9f завершился PASS2804.368с; он предшествует последним изменениям.

Tokenizer ordinary PFCCO tradeoff теперь подтверждён:5474→5462B,
319166→319442T нафиксированной полной нагрузке. Greedy сохраняется поумолчанию;
измеренный selectorstrategies пока разрабатывается отдельно. [Интеграция084](grace-integration-084/README.md).

Новый API MeasureContractStrategies интегрирован: off, greedy и PBQP,
точный измеренный ASM/BIN, отдельные Speed/Size и фиксированный порядок равенства.
На arena обе цели выбирают off: он равен PBQP — 1184 B / 3808 T;
greedy требует 1193 B / 3896 T. Memory hosts и вывод tokenizer в этом домене
не поддерживаются. [Интеграция086](grace-integration-086/README.md).

FAT12 disk ABI исправлен: byte-load из BC/DE в spill теперь сохраняет
живой AF; номер диска больше не заменяется последним байтом имени.
18 исходных native FAT12 случаев и125 ISA regressions проходят.
Memory-host теперь допускает действительно недостижимые extern с полным
исходником, сохраняя source/candidate reachability и hard ABI проверки.
[Интеграция087](grace-integration-087/README.md).
Новый полный core5fe165dd: HIR/MIR2/Nanz/VIR/CLI PASS; CLI425,234с.
[Полный лог и контекст](grace-integration-089/README.md).

FAT12 address hoist интегрирован отдельно от correctness fix: Grace Speed
13828→11202 T, +16 B полного модуля; default и Grace Size сохраняются.
Root independent ABI/memory/wrap/zero/fault-prefix проверки проходят.
Это cache-hit процедура с дисковыми адаптерами, не настоящий диск и не heldout PGO.
[Интеграция088](grace-integration-088/README.md).

## Следующие шаги

1. Проверять checked-local таблицы на дополнительных реальных случаях и
   измерять стоимость генерации/компиляции; calls/CFG/islands пока вне допуска.
2. Расширять независимое исполнение корпуса; для host ZVM и Agon определить
   требуемый runtime/target, сохраняя явные ограничения вместо фиктивного PASS.
3. Расширять измеренный invariant address hoist и автоматический выбор
   полезных кандидатов. Memory fusion отложена: порядок partial trap и
   liveness нельзя менять без явного доказательства.
4. Полный core5fe165dd и pipeline3e143f9f прошли на своих snapshot;
   отдельный свежий CLI запускается тремя процессами. Проверить соответствие
   всей цели по актуальным исходникам и точным scope каждого gate.
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

Последняя подтверждённая публикация — Sites 64, source `936cca8b`, compiler snapshot `2d09774a`. Она включает terminal core, four_ptrs, PFCCO arena, parallel CLI, C89 и CUDA audit. Indexed ENRT и byte-lane исправление появились позже и сохранены в repo/share. Новые runtime, main, PFCCO и исправления сохранены в архивных пакетах; policy-fixture `22e426de` следует после snapshot. Свежие sum_array, full30
и CP/M main собраны на странице current-state.html. Перенос фиксированных
вариантов опубликован; CLI gate, terminal pipeline и обновление трёх эталонов опубликованы
с исходными exit-кодами и независимым итоговым аудитом. Все внешние страницы и
артефакты сохранены локально с SHA256; исходные исполняемые файлы
восстанавливаются из опубликованного общего архива без потерь.

Аудит таблиц и отдельный оригинальный C89 utility интегрированы; root focused PASS 0.851 с. Дополнительные исторические таблицы найдены на шаре; импорт и параллельный полный CLI ещё исследуются. [Контекст](grace-integration-090/README.md).

Исторические assignments теперь можно импортировать через независимую повторную проверку; пять модулей на ENRT4v sample сохранили baseline размер. CUDA partial evaluator проверен на 512 случаях и сохранён на шаре. [Интеграция091](grace-integration-091/README.md).

Полный CLI snapshot0cbfeff7 прошёл тремя отдельными процессами: 71/71 PASS, 0 SKIP, 279.75 с включая сборку. Root независимо сверил discovery, partitions, run и terminal events. Самый тяжёлый отдельный SearchCLI тест — 270.36 с; текущая сумма времен тестов 399.51 с не является повторным serial benchmark. [Runner и замеры](grace-cli-process-shards.md).

Indexed ENRT5v conversion даёт triple module34→31B/direct66→54T/indirect123→111T. Pure byte-lane identity ускоряет FAT st_word220→93T и уменьшает модуль на23B, сохраняя byte-store эффекты. Свежие fullMIR2/VIR PASS12.391/2.178с. [Пакет092](grace-integration-092/README.md). Sites64 опубликован на snapshot2d09774a и целиком сохранён на шаре; indexed+lane опубликованы в repo/share позднее.

Compatibility внутри одного admission boundary теперь вычисляется один раз; все callback boundaries и executable hash сохранены. Root fullHIR PASS28.209с, equivalence PASS0.055с и factory/preflight pipeline PASS20.574с. [Пакет093](grace-integration-093/README.md).

Полный pipeline3e143f9f теперь terminal PASS2804.368с. Новая заранее frozen packed FAT12 нагрузка выявила native write_fat12 mismatch при raw/finalVM совпадении с независимым oracle; расследуется до выбора оптимизаций. [Пакет094](grace-integration-094/README.md).

Historical ENRT4v record5 теперь покрывает weighted13 SSA values через проверенный2-variable quotient; root all256 helper/caller original и factor9 PASS2.412с. Fresh showcase после lane/context PASS12.711с193/1. [Пакет095](grace-integration-095/README.md).

Первый packed FAT12 BIT-pattern fix интегрирован, root native parity PASS3.553с. Frozen odd write_fat12 остаётся FAIL: сдвиг черезHL затирает живое значение; workload не изменён. [Контекст096](grace-integration-096/README.md).

Новый exactfib_iter all256/u16 и reconciliation18tail: root pipeline PASS3.090с/catalogPASS2.143с;38limited VM/native paths,1nativeCPM. [Пакет097](grace-integration-097/README.md).

Root historicalcoverage118 functions PASS19.091с:9admitted,3smaller/4equal/2larger; largerневыбираются. Второй FAT indexedshift/liveHL fix проходит180native cases. GraceSize flags mismatch остаётся следующейcorrectnessзадачей. [Пакет098](grace-integration-098/README.md).

Третий packed FAT flags fix закрыт: root все108 native/VM cases PASS42.23с; полныйMIR2/VIR PASS. Новый ENRT checkpoint index подтверждает прежний triple выигрыш, rootnative PASS2.001с. Автоматический FAT selector ещё не заявлен. [Пакет099](grace-integration-099/README.md).

Selector ownership fix c0a7e49f удерживает отсоединённые HIR nodes/storage, предотвращая ложное shared-ownership после GC. Реальное разделение по-прежнему отклоняется; root GC/selector refusals PASS. [Пакет100](grace-integration-100/README.md).

Полный FAT public selector теперь rootPASS24.301с: Speed/Size выбирают baseline5572T9543B, exactsameBIN frozenheldout36calls11199T. Нового выигрышанет; большие/медленные альтернативы отвергнуты. [Пакет102](grace-integration-102/README.md).

Original ex19 localstruct return исправлен a8f7b4b5: root11568calls VM/native PASS2.046с/fullMIR2PASS12.318с. Каталог теперь39boundedVM/native paths, остальные исходныеentries не объявлены доказанными. Профильdemo печати сохраняется только вне reachable closure. [Пакет103](grace-integration-103/README.md).

Full CLI59db4b46 terminal PASS71/71,234.84с; roottest2jsoncoverage/SHAverified. Originalex19 allnine sourcefunctions boundedraw/final/nativePASS5.396с, generatedcallbackraw-onlyafterfinalinlining. [Пакет105](grace-integration-105/README.md).

Checked word multiply69566578 rootnativeall65536PASS1.681с/fullVIR2.356с: original112→63B; actualCALL1144→163T. Freshparser/HashMapfrozenheldoutrootPASS26.008с. Sitesv65published; itpredateswordintegration. [Пакет106](grace-integration-106/README.md).

Каталогexistingtableproofs reconciled: weighted/hash3/wordMUL больше неpending.42boundedVM/nativepaths95pending; не новыйimplementedcorpus. [Пакет110](grace-integration-110/README.md).

Originalboolmodule all12entries596736native+raw/finalVM rootPASS43.376с. ActualreturnA, notselectedF. Catalog43boundedpaths94pending. [Пакет112](grace-integration-112/README.md).

OriginalResult nineentries589842native/raw/finalrootPASS43.165с; taghelpersraw-only.44boundedpaths93pending. u32bitwisemiscompileunderrepair. [Пакет115](grace-integration-115/README.md).

## Проверки Option, SHA helpers и исправление u32

На `951c3ef9` интегрированы оригинальный Option (восемь исходных функций,
394 770 native вызовов в трёх режимах) и все 15 split-u16 SHA helpers
(69 480 входов, три режима). Root повторил raw/final VM и Z80: PASS за
29,276 с. SHA пакет проверяет арифметику, а не полный digest.

Исправлен реальный u32 AND/OR/XOR miscompile: операции теперь работают
по четырём байтам в обоих банках, сохраняют AF и восстанавливают банк.
Root проверки обычных исходников и принудительных размещений: PASS
(MIR2 0,977 с; pipeline 1,106 с). Подтверждённый динамический сдвиг
`1 << 256` всё ещё неверен на native и исправляется отдельным коммитом.
[Пакет интеграции](grace-integration-116/README.md).

Оригинальный enum match также прошёл все объявленные значения в трёх
режимах; [пакет](grace-integration-117/README.md). Полный MIR2 после
исправления u32 logic: root PASS 16,882 с.

## Полный счётчик сдвига и методы структур

Динамический `1 << 256` исправлен (`d0fe5ab0`): учитываются все байты
счётчика до записи scratch-регистра. Root native/raw/final проверки PASS,
полный MIR2 PASS 19,595 с. Семантика больших сдвигов сохранена по VM;
точной нормы в Book пока нет. Исходный50_impl_showcase также прошёл
все восемь функций и ограниченную независимую матрицу в трёх режимах.
[Пакет118](grace-integration-118/README.md).

В оригинальном14_error_propagation подтверждён новый native miscompile:
Carry выставлен, но A содержит аргумент вместо кода ошибки. VM пока не
поддерживает эти intrinsics. Исправление управления ошибками в работе;
готовность всех showcase к корректному исполнению не заявляется.

## Signed arithmetic и результат большого прогона

Все девять исходных функций signed_arithmetic прошли266240 raw/final
VM/native случаев в ordinary ContractOpt режиме; root PASS0,884 с.
Это одна конфигурация, деление на ноль отдельно.
[Пакет120](grace-integration-120/README.md).

Исторический полный pipeline8bf75e92 завершился FAIL2865,894 с.
Повтор пяти упавших тестов на текущем коде показал два уже исправленных
случая; диагностика отказа мутированного HIR исправлена отдельно. Два
отказа корректных workloads из-за generated LUT globals ещё исправляются.
Общий current fullsuite PASS не заявляется.

## Допуск generated LUT

Корректные popcount/ranged workloads снова проходят после точного
воспроизведения LUT из исходного MIR. Все пять прежних fullsuite failures
прошли root focused replay23,127 с; metadata tamper controls и native
workload guards сохранены. [Пакет121](grace-integration-121/README.md).
Это не новый общий fullsuite PASS.

## Wide edge copies и retained executable

Исправлен реальный u32 swap-loop miscompile: теперь edge copies сохраняют
оба банка до записи destinations. Root source/regression PASS1,614 с;
полный MIR2 PASS20,940 с. Отдельный popcount evidence теперь содержит
точный executable; root replay этого файла PASS.
[Пакет123](grace-integration-123/README.md).

## Явная семантика CY ошибок

Original14_error_propagation исправлен и прошёл983808 независимых
raw/final VM/native вызовов и полный main. Root PASS51,442 с; полные
HIR/MIR2/Nanz PASS. Code/status не теряются через CALL и cleanup.
[Пакет124](grace-integration-124/README.md). Поддержаны прямые проверяемые
u8/void вызовы; word/tuple/extern/nested CY формы и value-only profiles
пока явно отвергаются. Это не универсальная обработка ошибок.

## Error enforcement и аудит byte fusion

Исходный15_error_enforcement прошёл786432 raw/final VM/native исходов
в трёх режимах; полный main печатает105 и возвращает NC. Повтор14+15
на объединённом коде PASS88,975 с; HIR/MIR2/Nanz также PASS.
Каталог теперь51 bounded VM/native path,86 pending.
[Пакет125](grace-integration-125/README.md).

Byte-word fusion опционально даёт FAT helpers115200→89088 T и−146 B,
но независимый аудит выявил неверное доказательство смежности для
константы, усечённой по типу. Исправление в работе; текущие проверки
не позволяют объявить эту оптимизацию полностью проверенной.
Полные pipeline/CLI запущены на checkpoint до исправления.

## Исправления после полного gate

Старый pipeline708e77c5 завершился FAIL165,407 с: CY visitor обращался
к nil HIR до верификации. Исправлено01fd3576. Typed-constant ошибка
смежности fusion исправленаa7954042 в matcher, verifier и MemoryHost.
Root focused native/safety PASS7,151 с; новый полный pipeline запущен.
[Пакет126](grace-integration-126/README.md).

## Plasma: исходный helper и общий флаговый fix

Original plasma_attr ускорился793332→331508 T на1024 независимых вызовах;
полный BIN1555 B без роста. Это helper, не fullscreen renderer. Отдельно
исправлена потеря живого сравнения при умножении. Root native проверки
пройдены; [пакет127](grace-integration-127/README.md).

Sites v66 опубликован приватно и предшествует plasma. CLI708e77c5
завершился PASS365,129 с; новый pipelinea7954042 ещё работает.

## Третье семейство и направление VIR

Оригинальный Pascal SORT: выбор по трём frozen training-массивам;
те же выбранные488→486 B на восьми heldout дают164671→164275 T.
RootPASS8,110 с; полный исходник сохранён, BDOS main не исполнялся.
[Пакет131](grace-integration-131/README.md). Последующий повтор прежних
parser/HashMap/PFCCO heldout на currentкоде PASS25,262 с:
[пакет132](grace-integration-132/README.md). Это replay, не новоеblindfreeze.

Alice уточнила архитектуру: обычная компиляция без Z3, решатель только
для исследований/офлайн-генерации. Готовые таблицы применяются без solver.
VIR можно перепроектировать вокруг MIR-native проверяемых кандидатов;
[ADR0047](adr/0047-mir-native-candidates-and-offline-vir.md). WFC может
сокращать домены instructions/locations/flags/edges; его superiority
пока не доказана и он не заменяет текущий PBQP. Старый researchisland
эмиттер не имеет корректных joins и не допускается в productpath.

Флаговые scalar writers и полноразрядные u32/i32 сравнения исправлены;
unsigned widening extension также исправлено. [Проверки интеграции133](grace-integration-133/README.md)
и [интеграции134](grace-integration-134/README.md). Signed widening physical homes также исправлены; root 100 368 native случаев
PASS ([пакет136](grace-integration-136/README.md)).
Полный pipelinea7954042 жив, currentfullPASS по нему не заявляется.


## Текущая проверка оставшегося корпуса

Неизменённый widemath: все26callables, 2 248 035 nativecalls в трёх режимах,
raw/finalVM и orderedfield/RAM/SP/code модели; rootbounded replay PASS1,108 с
([пакет145](grace-integration-145/README.md)). Originalpromoted_arithmetic:
все9entries, 1 060 692nativecalls с raw/finalVM models, actualCALL/fullRAM.
Каталог после обоихproofs:53 independentpaths и84pending
([пакет146](grace-integration-146/README.md)).
Mixedunsignedwidth исправлен отдельно. Signedmixedcomparison native
согласуется с MIRSrcTy в следующем исправлении; frontendобщаяsignednesspolicy
не придумана по правилам C. LocalreturnedW8 lifetime за пределами немедленного
inspection проверяется отдельно; immediateproof не покрывает escapingvalue.
[Независимый аудит всей цели](../reports/2026-10-11-080-objective-completion-b1adbf09.md).

Весь опубликованный v67 сохранён локально и на шаре; exact manifest
подтверждён после устранения copy metadata ошибки
([пакет139](grace-integration-139/README.md)). Позднейшие исправления имеют
свежие repo/share пакеты; текущий сайт сохраняет свой более ранний checkpoint.


## Контракты вызовов: текущие исправления

Original Lizp functional: все8entries,398595nativecalls втрёхрежимах,
actualCallIndirect/targettrace и fullRAM/PCSP, rootPASS3,642с
([пакет148](grace-integration-148/README.md)). Каталог54independentpaths/83pending
([149](grace-integration-149/README.md)). Original lambdaszero-capture;
не заявляется новый environmentABI. Knownfunctioncallback mismatchedword/byte
ABI acceptance исправляетсядоemission, exactbeforepacketсохранён.

Signedcomparison повторяет actualMIRSrcTywidth, unsigned/Eq/Ne не изменены
([149](grace-integration-149/README.md)). Generalaggregateaddressclasses
исправлены, но originalnoninline returnedS lifetimeподтверждёнFAIL послеlaterCALL
([150](grace-integration-150/README.md)). CallerownedbufferABI реализуется
с publicentry/indirect/multi/recursion/effectguards; preliminaryfrozenfailures
устранены, полныйrootPASSпока не заявлен.

Currenttablejointmatrix44→20B/238→142T сохранена
([151](grace-integration-151/README.md)); ordinarytableapplication solver-free.
BoundedPFCCO/PBQPexhaustivecost controlsPASS
([152](grace-integration-152/README.md)).

## Проверка известных callback ABI

Интегрирован `8c9fa926`: сигнатура указателя-параметра, реально вызываемого
как функция, выводится из места вызова и распространяется через неизменяемые
алиасы и forwarding wrappers. Несовместимые известные цели и fallible CY
отклоняются до ASM; совместимые byte/word callbacks сохраняются.
[Исходный отчёт и проверки](../reports/2026-10-11-consumed-callback-signatures.md).
Общий возврат структур через caller-owned buffers и запрет свёртки
вызовов с памятью ещё в работе; их корректность здесь не заявляется.

Константная свёртка с памятью исправлена `4f58692b`; совместный callback/purity
root gate проходит ([пакет155](grace-integration-155/README.md)). General
struct-return buffers остаются незавершёнными.

Исходный Lizp DSP теперь имеет bounded независимую проверку всех пяти entry: 847 104 VM/native CALL в трёх режимах, без period0 и полного clamp domain. [Пакет157](grace-integration-157/README.md); актуальный каталог сохраняет все 194 пути.

MIR direct-call shape checks интегрированы `240db2ea`; оригинальные cellular функции имеют bounded VM/native proof всех трёх entries. [Пакет158](grace-integration-158/README.md), актуальный каталог: 56 independent/bounded, pending81.

Исходный scheme_test.lizp прошёл полные byte-домены всех 11 callable entries: 595 968 VM/native CALL, три режима. [Пакет159](grace-integration-159/README.md). Актуальный каталог: 57 independent/bounded, pending80.

Исходный PL/M abs_diff_fib.plm проверен полностью по byte-входам: 197 376 CALL в трёх режимах. [Пакет160](grace-integration-160/README.md), каталог58independent/bounded и pending79.

На исходном PL/M ABS_DIFF/FIB Grace уменьшает модуль 47→38 байт; полная нагрузка FIB(all256) 5 208 960→4 453 888 T (−14,50%). ABS_DIFF(all65536) остаётся 3 637 120 T. [Пакет161](grace-integration-161/README.md); это сравнение режимов, не новый training/heldout selector эксперимент.

Свежий compile/assemble gate9858face PASS14,165с:193PASS/1capacitySKIP. [Пакет162](grace-integration-162/README.md). General struct-return ownership/indirect canonical ABI ещё не интегрированы.

Полный core0f0953d9 завершился PASS: HIR/MIR2/Nanz/VIR, включая callback/purity/directcall fixes. [Пакет163](grace-integration-163/README.md). Struct-return WIP не включён.

Промежуточный caller-owned struct ABI и owning local copies интегрированы до58759553; callback metadata согласованы отдельно. Совместный focused gate PASS ([пакет165](grace-integration-165/README.md)). General multi-return/object adapter scope пока не закрыт.

FullCLI568b7d08 terminal PASS384,240с ([пакет164](grace-integration-164/README.md)); предшествуетstructbuffers. Current checked table focused gate9a3e38a3 PASS0,091с ([пакет168](grace-integration-168/README.md)).
