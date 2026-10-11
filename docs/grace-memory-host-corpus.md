# Корпус persistent SelfLanz workload

Проверены четыре новых workload неизменённого sourceSelfLanz через ту же
CLI-сборку1bd955: short-reuse12 calls, long-atoms9, nested-mixed9,
fresh-dominant20. На каждом workload реально измерены четыре бинарника
под speed иsize: всего8CLIcommands и32candidatepayload. Productioncode
иclosedhostpermissions не менялись.

Независимая workload-driven Pythonмодель привязана к известному original
source SHA; она воспроизводит все10 globals, полные node/string/source
ресурсы, результаты и ordered byteevents. Source-loop модель отдельно
совпала со всеми11 cumulative/per-call счётчиками иobservations. Три
неисполнявшихся main/print места остались0. Gas не приравнивается nativeT.

До nativeзапуска проверены вместимости: максимум723stringbytes,144nodebytes,
483sourcebytes; persistentstrings сохраняются, nodes/source reset поworkload.
Bothexactsites наблюдаются во всехfourcorpora; inputatoms<=255bytes.

| Корпус | Вызовы | Original T | Speed both T | Выигрыш |
| --- | ---: | ---: | ---: | ---: |
| Short reuse | 12 | 355595 | 349828 | 1,62% |
| Long atoms | 9 | 4401641 | 4216345 | 4,21% |
| Nested mixed | 9 | 1419255 | 1392784 | 1,87% |
| Fresh dominant | 20 | 849806 | 734242 | 13,60% |

В этихfour измеренныхcorpora speed выбралmask3both, size mask2append.
Фактические размеры четырёх вариантов2091/2212/2076/2197B сохранились
во всехcorpora; size2076B. Этиcosts относятся к даннымworkloads; новый
вход может изменитьвыбор, универсальныйwinner не заявляется.

Независимые semantic/profile/decision audits всех32candidate и8CLIoutputs
прошли. Финальный native audit и gzip-пакет прошли; перенесённый пакет
повторно прошёл независимую проверку32candidate и8decisions безCPUexecution.
Все контрольные суммы проверены; публикация после итогового коммита. Общиеcompilerтесты не повторяются:
производственный код не менялся сCLIstageb770f3be.

[Артефакты](grace-memory-host-corpus-artifacts/manifest.json).
