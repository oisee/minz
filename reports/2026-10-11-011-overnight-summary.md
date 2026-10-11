# Ночной итог Grace / PGO — 11 октября 2026

Актуально на 11:28 UTC. Цель продолжается: выбирать оптимизации по проверенному
профилю, от исходника через HIR/MIR до собранного Z80-бинарника. MIR VM служит
промежуточным oracle; итоговые варианты исполняются в Z80-эмуляторе с проверкой
результатов, памяти, порядка host events, размера и T-states. Проверка на
настоящем железе остаётся следующим уровнем.

## Готово

- Явные захваты HIR и сигнатуры вызовов, iterator lowering и исправления MIR/DJNZ
  проверены на VM и Z80. Неизменённый Self-Lanz работает через memory-host контракт.
- Привязка профиля к исходнику, workload и сборке; fresh replay; arena-first и
  append-copy; selector четырёх вариантов и Legacy CLI с сохранением ASM/BIN.
- Direct global u16 loads (`2908f18f`): pipeline PASS 1107.431 s.
- Явный typed unsigned compare opt-in (`1155fc66`): pipeline PASS 1078.565 s;
  HIR/MIR/Lanz/C89/CLI и Nanz без showcase PASS. Showcase: 31 прежнее падение,
  0 новых. Два full30 public прогона и два независимых аудита PASS.
  Default/API v1/обычный CLI используют Legacy. Public executable сохранён.

## Измерения на полном workload

30 последовательных вызовов Self-Lanz; одна fixed backend policy на четыре
кандидата. Это измерения конкретного workload, а не универсальный выигрыш.

| Кандидат | Legacy T / B | PreserveForSpeed T / B |
| --- | ---: | ---: |
| Original | 1135370 / 2000 | 1132449 / 2001 |
| Arena-first | 1135211 / 2121 | 1132290 / 2122 |
| Append-copy | 1079795 / 1978 | 1076874 / 1979 |
| Оба | 1079636 / 2099 | 1076715 / 2100 |

Два actual Speed selector запуска PASS 36.20 s, восемь кандидатных BIN и два
collection baseline сохранены. Независимые аудиты проверили все 300 native
наблюдений, результаты, 10 globals, 3 ресурса и по 3017 ordered host events
на baseline. Speed выбрал оба преобразования. Size mask 2 вычислен из этих
же сохранённых данных; полноценный CLI GoalSize прогон ещё впереди.

## Опубликовано и локально сохранено

Подтверждённая приватная публикация Sites — **версия 56**:
https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/

- [Совместный policy×mask поиск](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-policy-search.html)
- [Корпус с policy](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-policy-corpus.html)
- [Policy-aware selector](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-policy-selector.html)
- [Явный CLI v2](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-policy-cli.html)
- [Compare opt-in](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/unsigned-pair-policy.html)
- [Корпус](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/global-word-load-corpus.html)
- [Legacy CLI](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-cli-selection.html)
- [Отрицательный diamond эксперимент](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/normal-global-diamond.html)

Предыдущая офлайн-копия версии 52 проверена: **15158 файлов, 62 страницы**, все
SHA256 совпали, публикационные архивы читаются; отсутствующих локальных ссылок
и внешних зависимостей нет. Главная страница:
`/home/alice/dev/minz/artifacts/grace-reports/2026-10-10-resume/site/index.html`.
Исходники, профили, proofs, ASM/BIN, логи и Git bundles сохранены.
Selector и CLI опубликованы в приватном Sites 53. Whole offline sync версии53 завершён: **16588 файлов, 64 страницы**,
все SHA/публикационные архивы PASS, отсутствующих ссылок/внешних зависимостей
нет. Три исторических больших JSON опубликованы gzip; восстановленные
оригинальные bytes и SHA совпали, исходные файлы также сохранены локально.

Отдельный diamond forwarding эксперимент дал +1969 T / −2 B и остался в
изолированной ветке. Частный compare+reuse эксперимент дал −39231 T / −2 B;
reuse/hook в production не переносились. Оба результата опубликованы и сохранены.

## В работе

Association/selector v2 перенесён в рабочую ветку `0dceafab` и pushed. Whole
v2 envelope связывает policy, source profile и baseline; строгий loader до
source factory проверяет shape/schema/policy, вход ограничен 32 MiB. Все
четыре кандидата используют одну policy. Fresh replay обязательный.
Pretty JSON replay исправлен только нормализацией AllocationJSON после SHA
validation; первый отказ и executable сохранены. Артефакты: 270 tracked файлов,
коммит `ecda9d10`, локальный audit повторно PASS300.
HIR/MIR/Lanz/C89/CLI/Nanz прошли; showcase31same0new.
**Общий pipeline selector PASS 1241.045 s; этап закрыт.**

Следующий CLI v2 изолирован в `feat/memory-host-policy-cli`, source `c7c42795`,
inventory `6647fe0b`. Явная named policy, whole association collect/replay,
прямое сохранение выбранных ASM/BIN и SHA точных association/selection bytes.
Два независимых source review прошли. Admission/refusal тесты PASS .101 s;
short collect/replay/−o/wrong-policy PASS 47.206 s; старые Legacy проверки PASS
.041 s. Короткая проба имеет zero-body и не доказывает compare activation.
Все пять actual CLI запусков завершились exit 0 на сохранённом executable
`795f69cea3ed2cc2301a0a70aeabd29c70c3f0065273597e59cbdc58fc712c72`.
Actual Speed: mask 3, 1076715 T / 2100 B; actual Size: mask 2, 1076874 T /
1979 B. Короткий collect/replay и `-o` совпали побайтно; Legacy v2 контроль
прошёл. Независимые аудиты `5d7b4bcf` и `b639a913` проверили состояние, события,
профили, proofs и SHA. CLI перенесён в рабочую ветку `714f9e36` и pushed, четыре исходных файла
побайтно совпали с ревью. Текущая сборка:
`393af1d821477b991a0bf452f00c79bd6d77368c3cff9cde64faa1ee1b130808`.
Общий CLI gate PASS 69.418 s. Два fresh full30 integration запуска текущей
сборки завершились; независимые аудиты нового контекста прошли.
Все восемь свежих corpus CLI запусков завершились exit 0 на сборке393af1.
Четыре неизменённых workload, 50 вызовов, 32 кандидата; обе цели выбрали
append-copy,1979 B. T: 326090 / 3968554 / 1270168 / 696909. Оба независимых
аудита e02291a7 и22eacb5a прошли: corpus500 иproduction300, всего800native
наблюдений. Пакет684payloads закрыт и pushed81a02384, перенесён и relocatedauditPASS800.
Corpus опубликован в приватном Sites54; wholeoffline sync завершён:
**17965 файлов, 65 страниц**, все SHA/архивы PASS, отсутствующих ссылок и
внешних зависимостей нет.
Старые profiles/контексты остаются отдельными аналитическими сравнениями.
Следующий отдельный policy×mask API план a15b1b52/8d188be7 прошёл ревью: две
независимые whole association, общий ownership всех48callbacks, восемь
кандидатов, строгий Legacy-first tie. Реализация начата в изолированной постоянной ветке после ревью5d9941a7.
Дополнительно обязательна независимая сверка полного исходного HIR по
неизменяемым source bytes: retained SHA недостаточен. Код нового этапа
ещё не перенесён. Draft4af821fe исправлен e0521c1f: independent full AST +
metadata equality, exact Error/Failure identity и failedcandidate refusal.
Admission/validretainedSHA подмена PASS7.66s; focused48callbackGC/lifecycle
PASS314.10s, включая25/48late mutations иcross-policysharedgraphs. Дополнительные owned-input/actual short Size/tie проверки994280a2
PASS118.45s; missing/options preflightPASS4.37s. Harness1f117ae9 прошёл оба
ревью. **Full30 actual search TERMINAL PASS83.79s**, ровно48callbacks:
все восемь кандидатов совпали с ранее измеренными fixed-policy результатами.
Оба свежих collection baseline сверены с прежними public source/native
результатами; независимый cost audit PASS300 native observations.
Actual Speed выбрал PreserveForSpeed/mask3,1076715T/2100B;
Size, вычисленный из этих же сохранённых данных, выбрал Legacy/mask2,
1079795T/1978B. Full30 actual GoalSize нового поиска не выполнялся.
Точный executable2955420de33c6ce620f2298b0712ab537f0b66cd7fc540c545d344bff1b55966
и все inputs/search/objectives/ASM/BIN сохранены в
`/home/alice/dev/minz-artifacts/grace-checkpoints/memory-host-policy-search-evidence/`.
Короткий zero-body crossover контроль TERMINAL PASS113.95s на том же
executable,48callbacks. ActualSpeedpolicy1/mask1 566T/2122B; derivedSize
Legacy/mask2 663T/1978B. Zero-body не доказывает dynamic compareactivation.
Оба независимых аудита65167c16/66824fbc/2842284c прошли320observations
(full300+short20). API перенесён exact2files в44739f77 и pushed.
Общий pipeline TERMINAL PASS (exit0), exactexecutable/log сохранены; HIR/MIR/C89/CLI/Nanz PASS, showcase31same0new.
Broad gate нашёл цикл зависимостей frontend same-package tests→pipeline→Nanz→
frontend. Test-only исправление перенесено be43274d и pushed; Lanz/Lizp/PLM/Pascal
в рабочей ветке PASS .012/.012/.020/.011s. Независимая проверка полного HIR неизменна. Новый поиск с пояснениями и итоговым PASS опубликован в приватном Sites56; пакет292payloads сохранён
и trackedSHA проверен, relocatedauditsPASS320. Whole offline sync версии55
TERMINAL PASS:18570files66pages, всеSHA/архивы проверены,0missing/external.
Пакет и публикация завершены; общий pipeline прошёл. API-этап закрыт. Следующий
явный search CLI начат в изолированной постоянной ветке; короткий collect/replay/actualSize тест PASS251.97s; две финальные
source reviews прошли. Два full30 actual CLI запуска на сохранённом84e22
завершились: Speedpolicy1/mask3 1076715T/2100B, Sizepolicy0/mask2
1079795T/1978B. Оба saved audits PASS600. CLI перенесён exact5filesc9911c4e и pushed;
полный CLI gate выполняется. Свежая productionсборка0d6df9fb выполняет
два full30 integration запуска в отдельном новом context.

## Сохранение веток и решения

Эта сводка и завершённые этапы committed/pushed в
[feat/grace-hir-vm](https://github.com/oisee/minz/tree/feat/grace-hir-vm).
Нужные исторические /tmp ветки отправлены в remote `archive/2026-10-11/`;
повторный аудит обнаружил остаточный незакоммиченный memory-host каталог.
Он сохранён отдельным архивным snapshot `46120158` и pushed. Все 23
оставшихся /tmp worktree HEAD входят в сохранённую remote archive историю;
SHA перечислены в [манифесте](2026-10-11-011-archived-branches.json).
Все активные рабочие каталоги теперь вне /tmp:
`/home/alice/dev/minz-artifacts/grace-workspaces/`.
Есть дополнительные bare repository и bundles в `grace-checkpoints`.
Текущий WIP сохраняется отдельными commit/push; пользовательское изменение
`minzc/cmd/mzx/BUILD` не тронуто.

**Блокеров и решений Alice сейчас нет. Цель остаётся активной.**

Уточнения коллег проверены по коду: arena-first — первое byte compare peeling,
append-copy — вынос src/dst баз, PreserveForSpeed — регистровое unsigned16
сравнение; смены heap нет. Точные три эффекта аддитивны на этом workload,
но независимость backend стоимости не доказана. Pure Speed сохраняется
как минимум T без штрафа за байты; Pareto/byte-budget — отдельные режимы.
Публикация56 включает пояснения; wholeoffline sync56 PASS18973files66pages,
всеSHA/архивы проверены,0missing/external.

## Дополнение: завершён CLI совместного поиска

CLI `--memory-host-arena-append-search` интегрирован в `c9911c4e`.
Полный CLI gate прошёл; свежие Speed и Size запуски и две независимые
сверки прошли. Speed: 1 076 715 T / 2 100 B; Size: 1 079 795 T / 1 978 B.
Выбранные ASM/BIN сохраняются без повторной компиляции. Pure Speed
остаётся минимумом тактов без штрафа за размер.

Полный локальный комплект содержит 598 файлов с SHA256. Новый отчёт
готовится к публикации; последняя подтверждённая публикация — версия 56:
https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-policy-search.html

В работе: проверка фиксированных обучающих вариантов на четырёх известных
нагрузках, без повторного выбора. Это ретроспективная проверка переноса,
не слепой тест на новых входах. Первый код проходит два ревью до замеров.
Блокеров и решений Alice нет. Продолжаем цель.

## Публикация 57

Новые страницы опубликованы приватно через Sites:
- https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-search-cli.html
- https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/self-lanz-chain.html

Source сайта: `4bb196e74210b2d232fdb8fc41f03458858dd15f`.
Все исходные исполняемые файлы сохранены без потерь в общем архиве
с точным восстановлением SHA; архив разбит на части из-за Git object limit.
Полные оригиналы и 598 файлов CLI сохранены локально и committed/pushed.

Контроль переноса прошёл два ревью. Один заранее назначенный набор из
16 запусков идёт; первая нагрузка прошла независимую сверку. Полный
результат четырёх нагрузок ещё не заявляется. Полноценный CP/M main
требует отдельного startup/peek/poke/BDOS адаптера; нынешний function
стенд его не доказывает. Блокеров и решений Alice нет, продолжаем.

## Проверка переноса завершена

Один фиксированный набор прошёл за 91,40 s: 16 вариантов на четырёх
заранее назначенных известных входах. Два независимых аудита проверили
200 вызовов. Speed быстрее на 1,17 / 3,45 / 2,01 / 11,72%; Size также
быстрее на всех четырёх и занимает 1 978 вместо 2 000 байт.
Нового выбора на этих входах не было. Это проверка одного парсера
на известных нагрузках, не unseen holdout.

Пакет из 203 файлов и русская таблица committed/pushed; проверка после
переноса в root тоже PASS200. Сводная страница состояния готова;
публикация результатов и очистка повторов навигации в работе.
Следующий этап: отдельная COM-сборка полного main с реальными
памятью/BDOS под CP/M-эмулятором. Блокеров и решений Alice нет.

## Публикация 58 и следующий этап

Приватно опубликованы:
- https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-transfer.html
- https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/current-state.html

Навигация больше не повторяет одинаковые цели ссылок; исторические
страницы сохранены. Полный пакет переноса: 203 файла с SHA,
оба независимых аудита и root portable audit PASS200.

При подготовке полного CP/M main найдена следующая конкретная задача:
обычный литерал имеет префикс длины (Book v7 §18.1), тогда как main
читает его до NUL; это ошибка примера. VM хранит все литералы как
CString, а @print lowering не различает представления — это отдельные
ошибки компилятора. План и независимые ожидаемые байты готовятся;
полноценный COM успех пока не заявляем. Исправление строк и отдельный
CP/M startup/BDOS adapter — текущая работа. Решений Alice не требуется.

## Повторная сверка первоначальной ночной цели

Сохранённый sum_array пакет содержит profile/workload/model/decisions,
исходный HIR, raw/optimized MIR, ASM и BIN. Длины точно 0/1/5/16/64/255.
Совместимость профиля, evidence и решения совпадает; source SHA совпадает
с текущим исходником. Сохранённый selected BIN равен измеренному pointer
варианту. Независимый арифметический пересчёт всех 1022 измеренных строк
и шести workload результатов прошёл. Это аудит сохранённых данных,
не новое исполнение или replay на текущем компиляторе.

Исторический workload: 48 495 → 31 483 T; модуль 128 → 177 B.
Данные проверки: `2026-10-11-012-night-goal-evidence-audit.json`.
Расширенная цель реальных Z80 программ остаётся активной; следующий
конкретный шаг — правильные строки VM/Z80, затем полный CP/M main.

## Строки: исправление в отдельной ветке

`experiment/string-representation` вне /tmp: первый checkpoint `b989352c`,
исправления ревью `5bd9f775`. VM хранит C/S/L согласно спецификации,
S/L literal print lowering использует обычные циклы, native emitter
отвергает превышение длины. Проверки VM/parser проходят. Ревью нашло
совпадение внутренних имён с пользовательскими и перекодирование
байтов UTF-8 встроенным VM print.char; оба исправлены с регрессиями.
Финальный срез добавляет GC-safe обход рекурсивных типов и atomic
memory-limit refusal. Native gates готовятся; Z80 успех ещё не заявлен.
Блокеров Alice нет, цель активна.

## Финальный строковый gate

Код `c9c3e1dc` прошёл два независимых source review и совпадение
9 SHA; VM raw/optimized, storage и hygiene проверки проходят.
15 native случаев подготовлены. Первый run ещё не выполнялся: перед ним
исправляется test-only exporter, чтобы сохранить полный ASM со стартом,
точный BIN, globals, такты и errors. Проверенные production-файлы
остаются заморожены; повторных native запусков для упаковки не будет.

## Строки: native прошёл, общий gate ещё открыт

В рабочую ветку включён `964cb29f`: представления C/S/L совпадают с
Book v7 §18.1, counted print и вывод байта используют фактические
регистры. Исправленный запуск всех 15 Z80 случаев прошёл; сохранены
полный ASM, BIN, вывод, globals, PC/SP и такты. Два независимых аудита
проверили сохранённые данные и повторную сборку ASM без повторного CPU.
Первый неуспешный запуск сохранён отдельно и не переименован в успех.

Общий gate выявил четыре новых invalid ASM только через
CompileHIRWithOptions; CompileHIRSteps собирает те же программы.
Это текущая ошибка компилятора, исправление и регрессии в работе.
Основные HIR/MIR2 проверки прошли; полный CLI gate ещё исполняется.
Коммит `9329251d` исправляет устаревшее ожидание SString print и
пример метафункции: emit получает явный CString, без нового
автоматического преобразования ABI. Две затронутые проверки прошли.

Последняя публикация — Sites 58: transfer и current-state доступны
на https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/.
Новый строковый этап ещё не опубликован. Блокеров и решений Alice нет;
после закрытия общего gate продолжаем полный CP/M main.

Полный CLI gate завершился успешно за 335,963 s. Четыре новых ASM
падения связаны с подтверждённым столкновением номеров временных
регистров между функциями после allocation. Исправление и native
проверка второго пути ещё впереди. Логи успеха и ошибок сохранены
отдельно в docs/grace-string-parent-gate-artifacts с SHA256.
