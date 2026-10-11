# Ночной итог Grace / PGO — 11 октября 2026

Актуально на 08:31 UTC. Цель продолжается: проверяемая оптимизация реальных
программ от исходника через HIR/MIR до собранного Z80-бинарника. MIR VM —
промежуточный oracle; итоговые варианты исполняются в Z80-эмуляторе с проверкой
результатов, памяти, порядка ресурсных обращений, размера и T-states.
Настоящее железо остаётся следующим уровнем проверки.

## Завершено

- Исправления HIR/backend и opt-in PGO для sum_array, checksum, tokenizer и
  hashmap проверены на VM и actual Z80. История и отдельные ограничения
  сохранены в [handover](../docs/project/HANDOVER-2026-10-10-GRACE-PGO.md).
- Неизменённый Self-Lanz проходит закрытый memory-host контракт: ограниченные
  RAM-регионы, byte peek/poke, persistent calls и проверка частичного префикса
  до отказа. Его source-loop профиль привязан к исходнику, workload и сборке;
  загрузчик отклоняет несовместимые или неполные данные и повторяет исполнение.
- Реализованы отдельные arena-first и append-copy преобразования, совместный
  selector четырёх вариантов, typed precompile proofs и явный opt-in CLI.
  Решение использует фактические T-states/байты, сохраняет выбранные ASM/BIN.
- Последний полный pipeline: **569.037 s PASS**. После добавления CLI его
  общие тесты: **30.102 s PASS**. Showcase: **31 прежнее падение, 0 новых**.
  Последующие этапы меняли только workloads, артефакты и документацию;
  повторный полный pipeline для них не заявляется.

| Self-Lanz, 30 последовательных вызовов | T-states | Байты |
| --- | ---: | ---: |
| Original | 1221494 | 2091 |
| Arena-first | 1218395 | 2212 |
| Append-copy hoist | 1150124 | 2076 |
| Оба преобразования | 1147025 | 2197 |

Speed выбирает оба преобразования: **−6.10% T**. Size выбирает append-copy:
**−15 байт**. Production CLI/source: `b770f3be`; сохранённая сборка имеет
SHA256 `1bd95516f7b1ef1ba56c17822104936e2b73fe8bde5fe7368dd128811009f041`.

Дополнительный корпус: **4 workloads, 50 вызовов, 32 actual Z80 кандидата,
8 CLI решений**. Speed выиграл 1.62–13.60% в этих наборах; универсального
победителя не заявляем. Три коротких workloads действительно сменили speed
выбор на arena-only: **24 кандидата и 6 решений** проверены независимо.
Пять отказов CLI подтвердили unobserved site, typed host-denial prefixes и
отдельный gas limit; accepted selection и output directory не создавались.
Последний завершённый этап артефактов: `99f779cc`.

## Опубликовано и сохранено

Приватный Sites, подтверждённая **версия 47**:
https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/

- [Стоимость PC и функций](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-pc-cost.html)
- [Совместный selector](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-composition-selection.html)
- [CLI](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-cli-selection.html)
- [Корпус workloads](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-corpus.html)
- [Сохранённые профили и точные отказы](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-supplied-profiles.html)
- [Смена выбора и отказы](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-choice-boundaries.html)

Весь сайт сохранён офлайн: [локальная главная](../artifacts/grace-reports/2026-10-10-resume/site/index.html).
Проверены **13193 файла, 57 страниц**, SHA256 всех файлов, читаемость всех
публикационных архивов; потерянных ссылок и внешних зависимостей нет.
Исходники, профили, решения, proof records, ASM/BIN, логи, обе разные CLI
сборки и Git bundles сохранены локально.

## В работе

Проверка **supplied-profile CLI route закрыта**: authentic current профиль
принят (arena-only, 763 T / 2212 B), authentic stale и три явно synthetic
completion/count mutations отклонены с точными причинами, exit 1 и без output
каталога. Четыре accepted кандидата прошли независимые state/event/count/
proof/native SHA audits и проверку сохранённых файлов после переноса.
Это не failed-profile exporter; production code/exe не менялись.
Пакет `716a6c87` committed/pushed; приватная Sites версия 46 опубликована.
Полная офлайн-копия версии 46 проверена: 13044 файла, 56 страниц,
все SHA256 совпадают, все архивы читаются, отсутствующих ссылок нет.

Actual per-PC этап измерен и проверен: два прежних 30-call BIN/ASM/state/
T/steps совпали, сырые PC журналы сохранены, отдельная physical projection
распределяет все T по функциям и адаптерам. Экономия 74469 T полностью в
intern_region (71370 T) и region_match (3099 T). Cur239391 T и parse236919 T
неизменны. Пакет `154f4a95` и итоговый native saved-file review прошли;
приватная Sites версия 47 опубликована. Полная офлайн-копия проверена:
13193 файла, 57 страниц, SHA256 совпадают, все архивы читаются,
потерянных ссылок и внешних зависимостей нет.
диагностический hook остаётся только в изолированной ветке.

Backend direct global u16→DE/BC прототип дал measured gain на whole30:
original1221494→1135370 T,2091→2000 B; both1147025→1079636 T,2197→2099 B.
Независимые fullstate/events/native SHA аудиты прошли. Пять frozen source/test
файлов интегрированы; общий pipeline повторяется с timeout30m. HIR22.104/MIR5.174/
Nanz1.592/Lanz0.015/C895.336/CLI21.181 s PASS; showcase31same0new.
Новая production CLI dba34 имеет собственные свежие профили: full30speed/
size и short collect прошли, same-executable replay завершает проверку.
Final CLI saved-audit прошёл. Первый pipeline упёрся в 10m timeout и
выявил три устаревших baseline T/SHA assertions. Их reference обновлён
по независимо проверенному новому BIN; focused3tests4.173s PASS.
Полный suite повторяется с timeout30m; до его результата этап не закрыт
и не опубликован. Логи первого отказа сохранены.
Final suite с timeout30m продолжается, процесс активен. Отдельный
artifact-only corpus refresh на dba запущен: первые три speed-набора
выбрали append-only вместо прежнего both; полный32candidate аудит впереди.
Inputs и планы новых проверок committed/pushed в постоянных ветках.
Production checkpoint сохраняется как WIP; аудитированные старые этапы
остаются доступными. Диагностический PC hook в production не переносился.

## Сохранение веток и решения Alice

Сводка и завершённые этапы committed/pushed в
[feat/grace-hir-vm](https://github.com/oisee/minz/tree/feat/grace-hir-vm).
Нужные исторические /tmp ветки сохранены в remote `archive/2026-10-11/`;
точные SHA перечислены в [манифесте веток](2026-10-11-011-archived-branches.json).
Текущие ветки работы и ревью также committed/pushed; WIP не выдаётся за
закрытый этап. Активные рабочие каталоги находятся вне /tmp:
`/home/alice/dev/minz-artifacts/grace-workspaces/`.
Дополнительные bare repository и bundle — в `grace-checkpoints`.
Пользовательское изменение `minzc/cmd/mzx/BUILD` сохранено без изменений.

**Блокеров и решений Alice сейчас нет. Цель остаётся активной.**
