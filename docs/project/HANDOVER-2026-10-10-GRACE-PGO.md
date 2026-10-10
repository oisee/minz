# Grace/PGO handover — 2026-10-10

Продолжать в `/tmp/minz-grace-pgo-resume`, ветка `feat/grace-hir-vm`: самостоятельный локальный клон с доступными Git metadata. Исходное дерево `/tmp/minz-grace-hir-vm` сохраняет незакоммиченную копию collector; его `.git` указывает в read-only `/home/alice/dev/minz/.git`, поэтому коммит там невозможен. Основной `/home/alice/dev/minz` стоит на старом master; не переключать его и не трогать пользовательское изменение `minzc/cmd/mzx/BUILD`.

Последние закрытые этапы: `de07b86f` captures/signatures, `76ceefc9` iterator stage semantics/DJNZ/live A, `6f6a822e` input-HIR source sites. Последний этап повторно проверен после восстановления: HIR/MIR2/Nanz/pipeline/Lanz проходят, showcase baseline — 32 прежних failures.

Добавлен `hir.NewLoopProfiler`: отдельная инструментированная структурная копия HIR, raw MIR2 VM, entries/body entries и completed/failed calls, независимые snapshots. Production не меняется. Пример sum_array даёт 6/341, все 6 результатов проверены. Документ: `docs/grace-pgo-collector.md`. Новые directed tests и те же пять пакетов проходят; showcase совпадает с базой (32 failures, 0 новых).

Следующий ограниченный этап: compatibility fingerprint + refusing loader. Не считать input source hash полным ключом; dependency/typed IR/semantic/target и collector stage необходимы до применения cost policy. Guard/proof отдельно от profitability. Lineage через Grace/splitting/fusion и iterator-generated origins ещё не сделаны.

Sites checkout `/tmp/minz-grace-report-site`, builder `/tmp/build-grace-report-site.py`. Подготовленные iterators/pgo-sites страницы не были опубликованы после остановки usage limit. При восстановлении owner-only audience подтверждён Sites get_site (version 9), но source helper не мог разрешить git.chatgpt-team.site из sandbox; escalation не выполнен: auto-review service 404. Не обходить approval. Последняя опубликованная версия остаётся прежней. Полная подготовленная офлайн копия находится в `/home/alice/dev/minz/artifacts/grace-reports/2026-10-10-resume/`; манифест различает её с опубликованной версией. Старую копию `/home/alice/dev/minz-artifacts/...` не менять: вне writable roots.

Повторение gate: из minzc, GOCACHE=/tmp/minz-grace-gocache и Go `/home/alice/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin/go`: `test ./pkg/hir ./pkg/mir2 ./pkg/nanz ./pkg/pipeline ./pkg/lanz -skip '^TestShowcaseCompileAssemble$' -count=1`. Showcase отдельно, сравнивать множество failures с `docs/grace-site-artifacts/showcase.txt`.

## Следующий срез завершён: compatibility/loader

`profile_compat.go`: typed HIR fingerprint + all attached origins + explicit ProfileContext, schema v2, bounded strict JSON refusing loader. Без контекста snapshots диагностические и не загружаются. Harness IDs/dependency manifest объявляет вызывающий код, не auto-attestation; подробные границы в `docs/grace-pgo-compatibility.md`. Production PGO не включён. Directed tests (including imported origins и loop-free body edits) проходят; пять пакетов зелёные, showcase остаётся 32/0 новых. Следующая очередь: автоматически формировать harness identity/dependencies, затем lineage/guard events и cost policy.

## Срез automatic context

Nanz SourceUnits/SourceUnitsComplete записывает реальные input hashes и объединяет transitive imports/generated inputs. Mixed frontend imports помечены incomplete; API отказывает вместо заявления полноты. BuildAutomaticProfileContext хеширует текущий executable и переданные workload/hosts/resource bytes, module Target + explicit model. BuildProfileCompatibility проверяет все SourceUnits. Подробнее `docs/grace-pgo-context.md`. Следом controlled execution harness, lineage/guard events; production cost policy пока не подключена.
