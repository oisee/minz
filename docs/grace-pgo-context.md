# Автоматический контекст профиля

10 октября 2026. Продолжение [compatibility loader](grace-pgo-compatibility.md).

Nanz записывает `Module.SourceUnits`: логическое имя каждого фактически разобранного input → SHA256 его байтов. Manifest включает исходник без statements/циклов и transitive Nanz imports, а также generated Nanz inputs. При объединении разных байтов под одним именем parsing отказывает. `SourceUnitsComplete` показывает полноту frontend manifest. Imports других frontends сохраняют хеш прочитанного файла, но помечают manifest неполным: их собственные transitive inputs пока не отслеживаются этим механизмом.

`BuildAutomaticProfileContext(module, ProfileInputs)` автоматически использует этот manifest, хеширует текущий executable через `os.Executable` и прочитанные байты, workload/hosts descriptors и переданные ресурсы. TargetID сочетает `Module.Target` и явно выбранный cost/ABI model. Нет fallback на Git revision или строку версии compiler. Если manifest неполный/пустой, hash неверен или resource имя совпадает с source unit, API отказывает. `BuildProfileCompatibility` теперь сверяет все SourceUnits с dependencies, включая loop-free units; semantic HIR hashing не включает diagnostic manifest map напрямую.

Исполняемый файл хешируется целиком: любые изменения сборки инвалидируют профиль консервативно. Разные Go test binaries также получают разные IDs. Семантика collector имеет явный version tag; hash executable дополнительно покрывает изменения реализации. Hash executable не аттестует динамическое окружение/внешние библиотеки.

## API

```go
context, err := hir.BuildAutomaticProfileContext(module, hir.ProfileInputs{
    TargetModel: "cpm-z80-cost-abi-v1",
    Workload: workloadDescriptorBytes,
    Hosts: hostDescriptorBytes,
    Resources: map[string][]byte{"story.bin": storyBytes},
})
// После проверки err:
profiler, err := hir.NewLoopProfilerWithContext(module, context)
```

Resources — уже прочитанные байты, не пути к файлам, которые могут измениться между сбором ключа и запуском. Описание workload должно включать ordered calls/arguments, initial memory, gas/memory settings; описание hosts — их implementation/configuration, ports и внешние эффекты. Пустые байты означают явно пустое описание. API не сериализует Go closures и не подтверждает соответствие descriptor реальному запуску; harness обязан использовать те же inputs и не менять VM/hosts после построения ключа. Ресурсы, прочитанные позже без регистрации, не обнаруживаются автоматически. SourceUnitsComplete отражает декларацию frontend, не является security boundary.

## Проверки и границы

[Directed tests](grace-profile-context-artifacts/contracts.txt): независимый SHA256 executable, deterministic context/roundtrip, imported helper без loops (comment-only edit), stale workload/hosts/resource/target, resource/source collision, incomplete manifest. Предыдущие collector/loader tests также проходят. [Пакеты](grace-profile-context-artifacts/packages.txt) HIR/MIR2/Nanz/pipeline/Lanz проходят; [showcase](grace-profile-context-artifacts/showcase.txt) — прежние 32 failures, новых нет.

Production PGO не включён. Следом — контролируемый harness для выполнения объявленных inputs, lineage через HIR rewrites/lowering и guard outcomes. Затем можно связывать наблюдения с cost policy; raw VM counts не измеряют Z80 T-states, а совместимость не доказывает legality.

## Продолжение

[Контролируемый harness](grace-pgo-harness.md) теперь выполняет те же declared workload bytes, из которых строится context, с fixed no-external-hosts режимом. Обычный automatic context API остаётся доступен для custom harness.
