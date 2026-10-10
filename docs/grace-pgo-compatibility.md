# Проверка совместимости VM-профиля циклов

10 октября 2026. Продолжение [collector](grace-pgo-collector.md).

Добавлены `NewLoopProfilerWithContext`, `BuildProfileCompatibility` и `LoadLoopProfile`. Снимки используют schema `minz.input-hir-loop-observations/v2`; прежняя v1 и диагностические snapshots без контекста отклоняются. Production pipeline ещё не использует профиль для выбора Grace.

## Ключ

Ключ включает всё семантическое HIR модуля: concrete statement/expression/type tags, поля типов, bodies импортированных функций, initial bytes globals, strings, captures, ABI attributes и Target. Отдельно хешируются все приложенные statement origins. Диагностические warnings/assert execution accounting и compile-time assertion lists исключены. Ключ консервативен: изменение любой функции может инвалидировать профиль всего модуля. Кодирование типизированной структуры не использует адреса указателей; неизвестные представления или циклы в структуре приводят к отказу.

Harness явно передаёт `ProfileContext`: CompilerID, SemanticsID, TargetID, SHA256 workload и hosts, словарь dependency unit/resource → SHA256. Версии compiler/VM semantics и target costs/ABI необходимо менять при изменении соответствующего поведения. Словарь должен включать исходники без циклов, внешние ресурсы и данные; каждый приложенный source origin обязан иметь совпадающий dependency hash. HIR fingerprint дополнительно ловит изменения импортированных bodies без loops. Harness сам отвечает за полноту списка: API сравнивает объявления, но не аттестует исполняемый compiler, hosts или workload. CompilerID не вычисляется автоматически из Git.

## Refusing loader

Loader заново строит ключ и карту из текущего HIR и текущего context. Принимает только тот же schema/stage, точную карту и ключ, полное покрытие счётчиками известных site IDs, хотя бы один completed call и отсутствие failed calls/unmapped loops. Нулевые loops допустимы; body entries без entry недопустимы. Не требует entries ≤ completed calls: вложенный цикл или рекурсивный вызов может входить много раз.

JSON ограничен 8 MiB. Отклоняются неизвестные поля, duplicate object keys (также различающиеся только регистром), trailing values, обязательные поля с null/отсутствием, отрицательные/переполненные uint64 counts. Возвращаются независимо декодированные данные, counts сортируются по ID. Это проверка совместимости наблюдений, не доказательство истинности счётчиков и не разрешение переписывания.

## Проверки

[Directed tests](grace-profile-compat-artifacts/contracts.txt) проверяют round trip, детерминированность, независимость context/snapshot maps, изменение compiler/semantics/target/workload/hosts/dependency, semantic HIR/global init/SMC/Target, stale source hashes, imported origins и helper body без loops. Отрицательные cases покрывают malformed JSON и отказ от частичных/пустых/несовместимых наблюдений. Импортированный helper для collector вызывается по его окончательному mangled имени; module-qualified frontend call resolution отдельно не расширяется.

[Пакеты](grace-profile-compat-artifacts/packages.txt): HIR/MIR2/Nanz/pipeline/Lanz проходят. [Showcase](grace-profile-compat-artifacts/showcase.txt): прежние 32 failures, 0 новых. Обычная компиляция/генерация Z80 не изменена.

## Дальше

Автоматический сбор dependency manifest и identity реального harness; lineage через Grace/splitting/fusion; trip distributions и guard outcomes; затем cost policy. Эта версия не связывает raw VM event counts с Z80 T-states, не разрешает reuse между workload/target и не включает PGO по умолчанию. Проверки legality/guard/proof остаются независимыми.

## Продолжение

[Автоматический context builder](grace-pgo-context.md) теперь хеширует executable, зарегистрированные inputs и полный Nanz source manifest. Ручной ProfileContext API сохранён; исполнение hosts/workload по descriptor ещё не контролируется автоматически.
