# Контролируемый harness для профиля циклов

10 октября 2026. Продолжение [automatic context](grace-pgo-context.md).

`RunLoopProfile(module, ProfileWorkload)` сам создаёт VM, применяет initial global overrides, размещает ресурсы в детерминированном порядке и выполняет список calls. Публичная VM/host callbacks наружу не выдаются. Workload копируется через JSON; ключ строится из тех же байтов, которые исполняются. `BuildWorkloadProfileContext` позволяет заново построить ключ для загрузки сохранённого профиля, без исполнения.

## Описание нагрузки

Schema `minz.profile-workload/v1` задаёт target model, положительные MaxSteps/MaxMemory (до 65536), именованные byte resources, overrides globals и ordered calls. Аргумент — literal value либо имя ресурса и byte offset; pointer разрешён только для pointer-typed параметра. Offset равный длине ресурса допустим как one-past-end pointer; обращение за границы остаётся ошибкой VM. Размер global override должен точно совпадать с declared storage. SMC/capture/extern entrypoints, отсутствующие functions/resources и неоднозначные аргументы отвергаются до запуска.

MaxSteps — общий gas на все calls, включая instrumentation. MaxMemory учитывает globals/function symbols, instrumentation symbols и input resources; начальное переполнение отвергается явно. Calls разделяют память, globals и оставшийся gas. Results, final global bytes, final resource bytes и snapshot профиля возвращаются отдельными данными; caller-owned workload не меняется.

У первого harness фиксированный режим hosts: только внутренние collector hooks. Builtin/external hosts отсутствуют; попытка print/внешнего call заканчивается VM error без внешнего эффекта. MIR ASM/port I/O отклоняются до исполнения. Этот режим не предназначен для программ, которым нужны I/O, allocation hosts или внешний state. Production VM и compiler pipeline не меняются.

Ошибка исполнения возвращает partial ProfileRun вместе с error: успешные results до ошибки, final memory и профиль с FailedCalls. Loader отвергает такой профиль. Ошибка setup возвращает nil result. Counts являются наблюдениями input HIR, не target timing.

## Проверки

[Directed tests](grace-profile-run-artifacts/contracts.txt): ordered calls, независимый oracle results/final memory, shared globals, отсутствие изменения input bytes, loader roundtrip, изменения order/memory/gas/resource/global, setup refusals, partial gas failure и отказ builtin print без stdout. [Реальный sum_array](grace-profile-run-artifacts/sum-array-run.json) и [exporter](grace-profile-run-artifacts/export.go.txt): длины 0/1/5/16/64/255, суммы проверены формулой, 6 entries / 341 body entries, все calls успешны.

[Пакеты](grace-profile-run-artifacts/packages.txt) HIR/MIR2/Nanz/pipeline/Lanz проходят. [Showcase](grace-profile-run-artifacts/showcase.txt): те же 32 failures, новых нет.

## Следом

Lineage исходных sites через HIR rewrite/lowering и точные guard events; затем cost policy. Данный harness не устанавливает memory isolation между ресурсами (VM heap flat), не запрещает literal pointer values и не является sandbox для недоверенного кода. Mutation входного HIR/workload из другого goroutine не поддерживается. Runtime host traces и controlled I/O требуют отдельного контракта; measured Z80 execution и PGO legality/profitability пока не связаны.
