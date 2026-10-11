# Ночной итог Grace / PGO — 11 октября 2026

Общая цель остаётся активной: проверяемая оптимизация реальных программ от
исходника через HIR/MIR до собранного Z80-бинарника, с сохранением поведения,
памяти и обращений, измерением размера и T-states. MIR VM служит oracle;
проверяем также настоящий бинарник в Z80-эмуляторе. Железо — дальнейший этап.

## Что завершено

- Opt-in PGO измеряет варианты целых workloads и выбирает speed/size по
  действительным бинарникам. Реальные hashmap и tokenizer проверены по
  независимым результатам, памяти и traffic, включая persistent calls.
- Исправлены объявления/присваивания/fold integer widths и LetIn shadowing
  (`dbac514b`), bool→u8/u16 и повторные predicates (`d6e3a8d3`), сохранение
  живого HL при прямой записи byte-константы в global (`9533b4ad`).
- Последний полный pipeline прошёл за279.954 s; HIR/MIR/Nanz/C89 и CLI зелёные.
  Showcase:31 известных падение,0 новых.608 bool machine cases,14 source
  assertions; HL:12 source cases и4 machine cases. Бюджетные варианты
  скомпилированы заново; assembler подтвердил прежние ASM/BIN.

| Текущий workload / цель | T-states | Байты |
| --- | ---: | ---: |
| Hashmap / speed и size | 123708 | 965 |
| Tokenizer / speed | 2174363 | 4820 |
| Tokenizer / size | 2240918 | 4305 |
| Исходный Self-Lanz / baseline с peek/poke | 1221494 | 2091 |

Self-Lanz проходит30 последовательных вызовов, включая длины атомов255/256/257,
полные node/string arenas,10 globals и порядок byte events. Это baseline
с явными test-only адаптерами peek/poke; production parser PGO ещё не завершён.
Ошибочные исторические бинарники сохранены и не используются как speed baseline.

## Что опубликовано

Приватный сайт, подтверждённая версия37:
https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/

- [Локальные bindings и Self-Lanz](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/local-bindings.html)
- [Bool extension](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/bool-extension.html)
- [Сохранение HL](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/global-hl.html)

Полная офлайн копия: `artifacts/grace-reports/2026-10-10-resume/site/index.html`;
SHA256 манифест рядом. Версия37 сохранена и проверена:7875 файлов,47 страниц без потерянных
ссылок и внешних зависимостей; все архивы читаются.
Все исходники, профили, решения, ASM/BIN, логи и архивы сохраняются локально.

## Что в работе

Отдельный закрытый memory-host workload/контракт для исходного Self-Lanz:
ограниченные RAM-регионы, точный ABI, owned inputs/reset/persistence, проверка
каждого фактического byte access и частичного префикса до ошибки.
Старые no-host API продолжают отклонять extern/ASM/ports.

Замороженный WIP уже проходит focused HIR/VM и30 вызовов нового native runner,
но готовым контрактом не считается. Осталось закрыть владение factory-исходниками
и происхождение native-доступов к globals. Оптимизированный MIR после
PreallocCoalesce допускает повторные определения: его строгий SSA Verify
не отключаем; первый контракт сравнивает raw verified VM с fresh actual binary.

После baseline — отдельный host-aware сертификат arena-first сравнения строк
и измерение полного parser workload. PC профиль показывает intern_region334605 T
(27.39%); текущие typed copy/region/RW правила этот исходник не принимают.

## Что блокирует / решения Alice

Решения Alice сейчас не нужны. Технический незавершённый вопрос — подтверждение
происхождения global адресов в native runner; ослаблять проверки ради прохождения
не будем. Архивируем WIP как WIP и продолжаем реализацию.

По запросу Alice все нужные ветки и исторические эксперименты сохраняются
коммитами и push; активные рабочие копии переводятся из /tmp в постоянный каталог.
Пользовательское изменение minzc/cmd/mzx/BUILD не затронуто.

## Подтверждение сохранения перед продолжением

Ночной итог и завершённые изменения отправлены в GitHub:
[feat/grace-hir-vm](https://github.com/oisee/minz/tree/feat/grace-hir-vm).
WIP-контракт памяти сохранён коммитом058458cb и отправлен в
[feat/grace-memory-host-contract](https://github.com/oisee/minz/tree/feat/grace-memory-host-contract).
Ещё26 исторических веток отправлены под archive/2026-10-11/; отрицательные
эксперименты явно остаются историей, не approved production.
Remote refs сверены с локальными SHA. Первоначальный remote unpack failure
устранён повторным последовательным push с pack.window=0.

Постоянные активные копии теперь находятся вне /tmp:
`/home/alice/dev/minz-artifacts/grace-workspaces/compiler`,
`/home/alice/dev/minz-artifacts/grace-workspaces/memory-host`,
`/home/alice/dev/minz-artifacts/grace-workspaces/report-site`.
Полный локальный Git-архив и проверенный bundle28веток находятся в
`/home/alice/dev/minz-artifacts/grace-checkpoints/`.

## Дополнение: закрыт memory-host baseline

Новый публичный API выполняет неизменённый Self-Lanz с закрытым контрактом двух
byte-host операций.30 persistent calls проходят независимую модель памяти и
порядка обращений, а также аудит90 instruction-sidecar записей и ASM/BIN.
Стоимость прежняя:1221494 T /2091 B; выигрыш parser PGO пока не заявлен.
Полный pipeline289.268 s PASS; showcase31 прежнее падение,0 новых. Все16
обычных PGO CLI-вариантов сохранили бинарники предыдущего этапа.
Следом — source-bound профилирование и отдельный arena-first сертификат.
Решений Alice для этого шага не требуется; цель продолжается.

Отчёт memory-host опубликован через Sites, приватная версия38:
https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-baseline.html
Коммит этапа65f89589 запушен. Полная офлайн-копия8257 файлов проверена по SHA256,
48 страниц, потерянных ссылок и внешних зависимостей нет; архивы читаются.

Следующий этап уже в работе: отдельный source-loop collector и связь с native
baseline, веткаfeat/grace-memory-host-profile-association (`a9b1606f`).
Полный SelfLanz30 дал11 счётчиков, совпавших с независимой моделью; region_match
49/59. Проверены частичные host-denials и отказ gas. До production-интеграции
остались независимый review и общий прогон. Проба first-byte peeling сохранена
как отказ unsupported IX stack snapshot (`ce37bb32`); отдельная ветка
feat/memory-host-call-frame добавляет узкий compiler-owned контракт.
Координатору через Herdr сообщение не доставлено: серверi7 timeout.

## Последнее обновление: source-loop профиль опубликован

Этап2537b1e0 запушен: закрытый source-loop collector связан с native baseline.
Независимая модель совпала со всеми11 cumulative/per-call счётчиками;
полный pipeline296.585 s PASS, CLI/packages зелёные, showcase31same0new.
Приватная версия39:
https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-loop-baseline.html
Офлайн8399 файлов,49 страниц,0 потерянных ссылок/внешних зависимостей, архивы читаются.

Продолжается isolated arena-first first-byte peel: полный SelfLanz30
1218395 T /2212 B против1221494 T /2091 B (−0,254% T,+121 B).
33 924 raw-domain случая прошли; pre/post whole-module/origins validator
проверен.14 успешных граничных вызовов и4 prefix-denial workloads сейчас
проходят actualZ80. Measuredselector и durable production certificate ещё
следующие задачи; решения Alice не требуются.

## Обновление: измеренный arena selector

Закрыт selector двух точных вариантов region_match: speed1218395 T /2212 B,
size1221494 T /2091 B. Строгий профиль повторно исполняется, преобразование
проверяет независимый pre/post validator.33924 VM cases и36 граничных
наблюдений actual Z80 прошли; весь pipeline397.044 s PASS. Showcase31 прежнее
падение,0 новых; все16 CLI и4 бюджетных бинарника прежние.
Отчёт и доказательства сохранены локально; публикация следующей приватной версии
Sites выполняется после коммита. Рабочие ветки находятся в постоянном хранилище
и отправлены на GitHub.

Следом уже выполняется append-copy base-hoist. Изолированный полный workload
30 вызовов дал1150124 T /2076 B: −5,84% времени и −15 B кода. Память и
ordered host events совпадают с независимой моделью. До интеграции остаются
независимый proof/domain и общий прогон. Решений Alice не требуется.

Arena stage57322d17 отправлен на GitHub. Приватная Sites версия40 опубликована:
https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-arena-selection.html
Полная офлайн-копия проверена:9283 файла,50 страниц,0 потерянных ссылок
и внешних зависимостей; архивы читаются. Открывать:
/home/alice/dev/minz/artifacts/grace-reports/2026-10-10-resume/site/index.html
Append-copy: независимый validator исключает ресурсы, перекрывающие VM globals;
1286 rawVM случаев и42 actual-target запуска прошли. До measured selector
и production-интеграции остаётся отдельный шаг. Цель продолжается.

## Текущий шаг: append selector интегрирован для общего прогона

Семь новых compiler-файлов заморожены и перенесены в постоянную основную ветку.
Public selector подтвердил −5,84% T и −15 B для полного30-call workload;
оба goals выбирают hoist. Zero-body отдельный вызов сохраняет original731 T
вместо895 T. Независимые proofs/host/typedpayload аудиты и GC/cap/refusal gates
прошли. HIR/MIR/Nanz/C89/CLI зелёные; showcase31same0new; полный pipeline
выполняется. Исходники и194+ evidence файла сохранены локально.

Следующая изолированная работа — четыре actual-кандидата arena+append,
с отдельным proof ровно двух изменений и проверкой взаимодействия.
Решений Alice не требуется; текущая публикация40 остаётся подтверждённой.

Append production полный pipeline прошёл за462.479 s. Всеобщиепроверки
завершены: HIR/MIR/Nanz/C89/CLI PASS, showcase31same0new. Этап закрывается
коммитом и следующей приватной публикацией; исходники/API/backend неизменны
с замороженного7-file checkpoint. Следующий combined prototype прошёл
независимые host/native аудиты четырёх вариантов: both1147025 T /2197 B
(−6,10% T), append самыймалый2076 B. Отдельный combined selector реализуется.

Append stageaaab134c запушен. Приватная Sites версия41 опубликована:
https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-append-selection.html
Полная офлайн-копия синхронизируется; опубликованный исходный набор7 файлов
сверен по SHA256 с интегрированным компилятором.

Combined selector финальный isolated public E2E прошёл за33.789 s: speed
выбираетboth1147025 T /2197 B, sizeappend1150124 T /2076 B. Каждому
из четырёх вариантов соответствует typed proof;24 factory calls с GC на
каждом режиме прошли. Независимый аудит пакета и production-интеграция впереди.
