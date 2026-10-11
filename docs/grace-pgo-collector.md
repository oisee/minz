# Наблюдения циклов input HIR на MIR2 VM

10 октября 2026. Продолжение [карты мест](grace-pgo-sites.md).

`hir.NewLoopProfiler` проверяет input HIR, строит карту исходных statement loops и создаёт отдельную копию структурных блоков с host-событиями. Обычный `LowerModule` и production pipeline не меняются. Инструментированный MIR исполняется без оптимизаций только на MIR2 VM; его нельзя подавать в Z80 codegen или Grace. Счётчики не являются измерением T-states или времени.

`LoopProfiler.Call` накапливает наблюдения, `Snapshot` выдаёт независимую JSON-сериализуемую копию карты и счётчиков. Через `VM` можно настраивать пользовательские hosts, память и gas limit. Для учёта completed/failed calls вызывайте метод profiler, а не `VM.Call` напрямую. API рассчитан на последовательное исполнение.

## События

- `entries`: управление достигло statement loop, до вычисления его границ/указателя/условия. Нулевой цикл тоже имеет один entry. Ошибка при вычислении границ оставляет entry в частичном профиле.
- `body_entries`: управление вошло в тело, до его первого пользовательского statement и до foreach load. Break, continue, return или trap внутри тела не отменяет это событие.
- `completed_calls` / `failed_calls`: исход каждого вызова через profiler. После ошибки сохраняются частичные счётчики; это не полный trip count. Ошибки пользовательских hosts тоже остаются ошибками.

События не добавляют writes в память программы. Имена private hosts выбираются без коллизий с declarations и прямыми source calls. Snapshots содержат исходные source hashes и schema/stage; отсутствующие origins отражены в `unmapped_loops`. Реестр не приписывает координаты iterator-generated или expression loops.

## Проверенный пример

Исходный `examples/nanz/01_sum_array.nanz`, длины 0, 1, 5, 16, 64, 255: **6 entries, 341 body entries, 6 completed calls, 0 failed calls**. Каждый результат сравнен с независимой формулой суммы `n*(n+1)/2 mod 256`. [JSON](grace-profile-artifacts/sum-array-profile.json), [воспроизводимый exporter](grace-profile-artifacts/export.go.txt).

Направленные тесты покрывают нулевые циклы, break/continue, вложенные циклы, for-range/for-each, эффекты global memory и сравнение с обычным MIR2 VM, отсутствие изменения HIR, независимость snapshots, частичный gas failure, отсутствующие origins и коллизии имён hosts. [Лог](grace-profile-artifacts/contracts.txt). [Пакеты](grace-profile-artifacts/packages.txt), [showcase](grace-profile-artifacts/showcase.txt): те же 32 падения, новых нет.

## Следующий этап и ограничения

Это collector наблюдений input HIR, ещё не профиль выбора оптимизаций. Instrumentation расходует VM gas и меняет `StepsExecuted`; эти величины нельзя сравнивать с обычной программой как ускорение. Не реализованы гистограммы trips, guard hits/misses, lineage через Grace/splitting/fusion, полный dependency/semantic/target fingerprint и refusing loader. Snapshots нельзя переиспользовать для оптимизации только по site ID/source hash.

Следующий шаг — определить полный ключ совместимости и loader, который отвергает неизвестную schema/stage, неполные или устаревшие данные; после этого связывать профиль с cost policy, сохраняя независимые legality guards/proofs.
