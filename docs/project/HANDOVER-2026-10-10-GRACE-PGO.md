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

## Срез controlled harness

RunLoopProfile/BuildWorkloadProfileContext (profile_run.go) строят context из owned workload copy и исполняют exactly declared calls/initial state/limits. Resource pointers symbolic; globals exact-sized; no external hosts, no ASM/ports; fixed internal collector hooks. Runtime failures возвращают partial snapshot with failed calls, setup errors — nil. Реальный sum_array снова 6/341, sums oracle. Directed tests проверяют results/final memory/shared globals, input immutability, loader invalidation order/memory/gas/resources/globals, refusals. Следом lineage и guard events; production PGO не включён. Документ docs/grace-pgo-harness.md.

## Ночной sum_array срез

profile_lineage.go + raw MIR LoopOrigins; guard eligibility counters успешных прямых точных sum calls (не branch instrumentation); profile schema v3. profile_policy.go strict-loads compatible observations, exact HIR/function/target model, big-int totals, conservatively original on misses/incomplete guards. ApplyProfileDecisions seeds lineage and runs existing Grace legality/guard. TestProfileGuidedSumVMZ80 executes original/pointer/selected for six lengths, independent sums + memory checks, actual ABI assembly/emulator, saves all artifacts. Full workload 48495→31483 T-states, 128→177 module bytes; short/size original. Next: fallback/full-domain measured model, automatic model registration and CLI/pipeline selection, corpus, MIR pass lineage. General user goal remains active; do not declare broad goal complete from this one-function slice.

Requested CLI subagent via `codex --profile azure`: normal start initially failed read-only app-server state; temporary CODEX_HOME with symlinked existing config allowed startup, but provider connections kept reconnecting. Escalation explicitly requested by user failed auto-review service 404, not executed. Herdr socket also Operation not permitted. Local VM/Z80 work unaffected. No completed independent agent review claimed.

## Full access / CLI reviewers / pipeline

Full access получен. Два reviewers реально завершили codex exec (azure read-only и default subscription read-only). Отчёты /tmp/minz-pgo-review-azure.md и /tmp/minz-pgo-review-z80.md. Их P1 findings: internal-call guard coverage (исправлен entry hook), stale decisions (полный compatibility recheck), stale compiler/options model (executable/semantics + pipeline compilation key), re-Grace override original (sticky module policy). Found existing u16 foreach truncation delegated default Codex workspace-write; fixed lower.go and foreach_wide_count_test.go, tests source VM/Z80 pass requested domain. Callback inlining lineage остаётся открытым.

Cost domain теперь 1022 cases including exact guard equality and wraps; branch-target observation and memory/code/stack overlap checks. Equality fast extra +7 cycles, fallback 208+141*n. Profile schema v4; models include equality/fallback costs, compiler/semantics. ProfileSelection wired to both pipeline entrypoints, Steps returns decisions. Need CLI collection/selection flags and model registration, broader corpus; general goal remains active.

## Callback-inline attribution

`ebb530c7` сохраняет lineage при renameStmt/renameBlock inlining callback. Clones while/range/foreach/switch блоков получают attribution в частной карте lowerer; input metadata не мутируется. Regression: visit callback с while встраивается дважды в main, обе raw MIR origins совпадают с SiteID visit, VM result 9. HIR/Nanz gate и range/foreach/PGO pipeline tests проходят. CLI/automatic measured model сейчас реализуются двумя реальными codex exec workspace-write подагентами; задача общая остаётся активной. Sites source helper снова доступен; подтверждён owner-only доступ, live пока version 9.

## Automatic measured model + CLI

Два codex exec подагента завершили implementation CLI и measured API; parent объединил в `--hir-profile-auto DIRECTORY --hir-profile-function NAME --hir-profile-auto-workload FILE`. Exact one mapped u8 sum site only. MeasureLoopCostModel компилирует original/pointer с текущими options, измеряет 1022 сценария VM/Z80, выводит coefficients из rows и отказывает nonlinear. Evidence SHA256 включает compiled binaries и per-case execution SHA. CLI сохраняет profile/run/workload/model/evidence/decision + HIR/MIR/ASM; default assembler compact bin origin0x8000 совпадает с measured binary.

Реальный built CLI `/tmp/minz-grace-pgo-cli` прогнал source sum_array: pointer 177B; profile totals 48495→31483. Повторное применение saved profile/model дало побайтно тот же bin. Osize отдельная fresh measurement выбирает original128B. Showcase32, no new/resolved. CLI directed tests и measurement tests проходят. Полный cmd/minzc suite имеет прежние callable-alias failures (`reassignment of callable alias fp is unsupported`); не заявлять весь CLI suite зелёным. Артефакты docs/grace-pgo-cli-artifacts. Документ docs/grace-pgo-cli.md.

Дальше общая цель: multi-site/function selection, обычные caller workloads и corpus, MIR transform lineage. Не считать exact sum runner завершением общей цели. Live report pending refresh после этого коммита; офлайн копия и SHA manifest обновляются отдельно.
