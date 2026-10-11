# Ночной итог Grace / PGO — 11 октября 2026

Актуально на 10:02 UTC. Цель продолжается: выбирать оптимизации по проверенному
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

Подтверждённая приватная публикация Sites — **версия 52**:
https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/

- [Compare opt-in](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/unsigned-pair-policy.html)
- [Корпус](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/global-word-load-corpus.html)
- [Legacy CLI](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-cli-selection.html)
- [Отрицательный diamond эксперимент](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/normal-global-diamond.html)

Полная офлайн-копия версии 52 проверена: **15158 файлов, 62 страницы**, все
SHA256 совпали, публикационные архивы читаются; отсутствующих локальных ссылок
и внешних зависимостей нет. Главная страница:
`/home/alice/dev/minz/artifacts/grace-reports/2026-10-10-resume/site/index.html`.
Исходники, профили, proofs, ASM/BIN, логи и Git bundles сохранены.
Новый selector и CLI пока не объявлены опубликованными.

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
**Общий pipeline этого этапа ещё выполняется, успешный итог не заявляется.**

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
Общий CLI gate и два свежих full30 integration прогона ещё выполняются.
Следом готовится тот же корпус из четырёх workloads с новой явной policy;
входы проверяются до новых запусков, старые профили не переподписываются.

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
