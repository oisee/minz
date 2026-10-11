# Совместный выбор policy и source transformations

Новый opt-in API `MeasureMemoryHostArenaAppendPolicySearch` измеряет восемь
сочетаний: Legacy / PreserveForSpeed × original / arena-first / append-copy /
оба. Обычный компилятор и fixed-policy CLI сохраняют прежнее поведение.
Результаты относятся к неизменённому `self_lanz_parser.nanz` и указанному
workload, а не к произвольным программам.

## Что меняют варианты

`arena-first` — историческое неудачное название выделения первого сравнения
в `region_match`. Для длины 0 возвращается 1 без чтений; первый байт областей
сравнивается отдельно, для длины 1 сразу получается результат. Оставшийся
цикл начинается с индекса 1. Стратегия выделения памяти не меняется.

`append-copy` выносит два неизменных адреса перед циклом копирования новой
строки в `intern_region`: `src_buf + start` и `str_bump + 1`. Цикл прибавляет
индекс к готовым базам вместо повторного вычисления сумм. Чтения/записи
байтов, изменение `str_bump` и результат остаются прежними. Это вынос баз
адресов, а не другой способ копирования или другая host-куча.

`PreserveForSpeed` — backend-политика unsigned 16 compare. В проверенных
регистровых условиях она использует последовательность сравнения, сохраняющую
пары регистров, вместо прежней общей последовательности. Она относится
к машинному коду, а не к управлению памятью.

Сама идея выноса баз применима шире, но нынешнее source-bound правило
распознаёт и доказывает конкретную форму `intern_region`. Произвольные циклы
копирования автоматически этим контрактом не охвачены.

До caller factory API принимает две обязательные whole association v2,
проверяет их policy/source/workload и независимо разбирает сохранённые source
bytes. Полный HIR, ABI, origins и metadata сверяются с этим reference. SHA
исходника сам по себе недостаточен. Графы удерживаются живыми на всех 48
callbacks; source sharing, поздние мутации и failed candidates дают отказ
без частичного победителя. Каждый fixed-policy selector выполняет fresh
replay и source/native checks. Search сохраняет оба полных selection и
восемь ссылок policy/mask; стоимость берётся из проверенного native payload.

Строго сравниваются фактические T-states или длина BIN. При равенстве основной
стоимости сохраняется более ранний кандидат: Legacy masks 0..3, затем
PreserveForSpeed masks 0..3. Cross-policy observations сохраняют все поля,
включая Error/Failure; допустимое отличие Steps проверяется отдельно.

## Измерения

Full 30 actual Speed search PASS 83.79 s; ровно 48 callbacks. Два свежих collection
baseline совпали с прежними public source/native результатами.

| Mask | Legacy T / B | PreserveForSpeed T / B |
| --- | ---: | ---: |
| Original | 1135370 / 2000 | 1132449 / 2001 |
| Arena-first | 1135211 / 2121 | 1132290 / 2122 |
| Append-copy | 1079795 / 1978 | 1076874 / 1979 |
| Оба | 1079636 / 2099 | 1076715 / 2100 |

Actual Speed выбирает PreserveForSpeed + оба преобразования, 1 076 715 T / 2 100 B.
Size, вычисленный из сохранённых BIN, выбирает Legacy + append-copy,
1 079 795 T / 1 978 B. Full 30 actual GoalSize нового API не выполнялся.

Короткий zero-body контроль PASS 113.95 s, 48 callbacks: actual Speed выбирает
PreserveForSpeed + arena-first, 566 T / 2 122 B. Derived Size выбирает Legacy +
append-copy, 663 T / 1 978 B. Это отдельный контроль переносимости/search; нулевые
body counters не доказывают dynamic compare activation.

## Практический обмен скорости на размер

Три недоминируемых варианта полного workload: Legacy + append-copy
(1079795 T,1978 B), PreserveForSpeed + append-copy (1076874 T,1979 B),
PreserveForSpeed + оба (1076715 T,2100 B). Остальные хуже одного из этих
вариантов по обоим показателям.

Последний переход экономит 159 T (0.0148%) за 121 B (+6.11%). Текущий Speed
минимизирует только T-states: это не рекомендация, что такой обмен всегда
разумен на машине с ограниченной памятью. PreserveForSpeed + append-copy
экономит 5.152% T и 21 B относительно исходного Legacy, почти достигая минимальных
тактов. Дальнейший интерфейс должен показывать весь Парето-фронт; отдельная
цель «минимум тактов при лимите байтов» требует собственного явного контракта.

Четыре ранее проверенных нагрузки парсера полезны, но победитель на каждой
выбирался отдельно. Это не проверка переноса победителя на отложенные входы.
Следующий эксперимент должен выбрать вариант на training workload, заморозить
его и проверить поведение/стоимость на заранее выбранной held-out нагрузке
без повторного выбора. Результаты относятся к конкретным входам.

## Аддитивность в этих измерениях

На данном полном workload эффекты точно складываются во всех восьми
вариантах: PreserveForSpeed −2921 T / +1 B; выделение первого сравнения
−159 T / +121 B; вынос баз копирования −55575 T / −22 B. Это проверенное
свойство сохранённых результатов, а не гарантия для других сборок.

Разные HIR sites могут взаимодействовать через распределение регистров,
ABI вызовов и размещение переходов Z80. Статическое непересечение изменений
не доказывает аддитивность машинной стоимости. Возможное сокращение поиска
до базы и одиночных вариантов требует отдельного контракта независимости;
итоговую выбранную комбинацию всё равно нужно собрать и измерить.

Профиль ранних несовпадений поможет объяснять пользу выделения первого
сравнения. Нынешний zero-body контроль выигрывает от возврата при длине0,
а не от несовпадения первого байта; соответствующие динамические счётчики
пока не собирались.

Чистый Speed сохраняется как минимум тактов без штрафа за размер. Парето,
лимит байтов и будущие цели добавляются отдельно. Включение backend-политики
по умолчанию требует проверки на других программах; сейчас она opt-in.
Обобщение выноса адресов должно учитывать неизменные операнды и эффекты
вызовов над globals, сохраняя порядок byte reads/writes и wrap семантику.

## Проверки и артефакты

Focused admission/valid retained-SHA altered-body PASS 7.66 s; forcedGC/lifecycle
и late mutations 25/48 PASS 314.10 s; owned-input/actual short GoalSize/tie
PASS 118.45 s; missing/options preflight PASS 4.37 s. Первое падение неправильного
fixture и его executable сохранены; исправленный negative сохраняет валидные
HIR origins. Synthetic ranking tests проверяют ordering, а не подлинность
машинных замеров.

Два независимых saved audits и повторная сверка перенесённого пакета прошли:
300 полных + 20 коротких observations, все 11 profile counters, память/ресурсы/globals,
ordered events, source/precompile proofs, association bytes/SHA, ASM/BIN,
MIR/allocation/sidecar/call-frame SHA. CPU при аудите не запускался повторно.

[Пакет и manifest](grace-memory-host-policy-search-artifacts/manifest.json)
содержит точные inputs, complete searches, все 16 кандидатных BIN, ASM/MIR,
логи, independent auditors и четыре retained executables. Большие payloads
сохранены как gzip; auditor читает точные восстановленные bytes. Final
executable SHA 2955420de33c6ce620f2298b0712ab537f0b66cd7fc540c545d344bff1b55966.
[Полный audit](grace-memory-host-policy-search-artifacts/relocated-full-audit.json)
и [short audit](grace-memory-host-policy-search-artifacts/relocated-short-audit.json).

API перенесён `44739f77`, test-only external frontend packages `be43274d`
устраняют integration import cycles без изменения production source gate.
HIR/MIR/C89/CLI/Nanz и affected frontend tests PASS; showcase 31 same 0 new.
Общий pipeline TERMINAL PASS (exit 0), полный лог и exactexecutable сохранены
в parent-gates пакета. API-этап закрыт; overall цель продолжается.
Следующий шаг после gates — отдельный явный search CLI с paired collect/replay
и exactselectedASM/BIN.
