# Проверенный выбор arena и append вместе

Новый ограниченный API сравнивает четыре actual Z80 варианта неизменённого
SelfLanz: original, arena-first peel, append base-hoist и оба. Принимает
фабрику оригинального HIR, closed memory-host workload, строгий serialized
source-loop профиль и MaxSites=2. Обе точные исходные петли должны быть
наблюдены; полный профиль повторно исполняется. Предоставленная transformed
фабрика, assembly или предсказанная стоимость не принимаются.

Каждый вариант приватно строится и проверяется независимым aggregate
precompile validator. Он допускает ровно два проверенных изменения функций,
проверяет весь остальной модуль, closed VM storage исключение и union source
origins. Одиночные validators остаются строгими и отклоняют другое изменение.
Typed proof каждого из четырёх вариантов связывает choices, два SourceSites,
original/actual FunctionSiteMap, natural baseline compatibility и ActualSites.
У final MIR нет выдуманного HIR сертификата или унаследованных счётчиков.

Первое original измерение используется из свежей association. Затем три
оптимизированных варианта исполняются через закрытый baseline. Всего24 вызова
пользовательской фабрики; каждый оригинал остаётся проверяемым sentinel,
все старые AST/backing allocations удерживаются перед destructive prepare
и через GC. Проверки позднего изменения source-origin на фазах6/12/18/24
и настоящего shared AST между вариантами прошли.

Final public E2E33.789 s PASS под обоими goals с GC на каждом callback:

| Вариант | T-states | Байты |
| --- | ---: | ---: |
| Original | 1221494 | 2091 |
| Arena | 1218395 | 2212 |
| Append | 1150124 | 2076 |
| Both | 1147025 | 2197 |

Speed выбирает both: −74469 T (6,10%). Size выбирает append: −15 B.
Стоимость каждого варианта измерена целиком; сумма выигрышей не используется
для решения. Равная основная стоимость сохраняет более ранний вариант в
фиксированном порядке0..3; goal не меняет compiler options.

Все8candidatepayload двух goals прошли независимые host/state/events и typed
native audits. Исходник,10 globals,3 full resources и порядок byte событий
совпадают; native sidecar/finite frames/allocations/MIR/ASM/BIN связаны SHA256.
Прототип fouractual7.392 s сохранён отдельно: его артефакты не заявляют
сериализованный aggregate certificate; finalselector создаёт такой record
до compiler preparation. Shared VM/actualdomains предыдущих этапов сохранены.

Шесть новых файлов интегрированы, существующийbackend не изменён. HIR22.528 s, MIR5.758 s, Nanz1.584 s, C895.499 s и CLI20.432 s PASS;
showcase31same0new. Полный pipeline прошёл за569.037 s. Все необходимые общие проверки
завершены; публикация выполняется после финального коммита.

[Артефакты](grace-memory-host-composition-artifacts/manifest.json).
