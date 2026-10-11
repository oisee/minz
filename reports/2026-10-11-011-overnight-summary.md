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
