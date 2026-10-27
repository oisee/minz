# Работа возобновлена через Z.ai workers

Alice после завершения книги явно попросила решать оставшийся tail через
`codex-z + claude-z`. Это новое указание заменяет паузу из handover. Legacy STOP
marker не удаляется: daemon/Herdr не используются, работа запущена явно.

Оба wrapper найдены в ~/.zshrc. Минимальный headless smoke с low effort:
Claude→Z.ai ответил READY за2.4s, Codex→Z.ai за11.5s. Это подтверждает доступность
каналов на момент проверки. Codex предупреждает о fallback model metadata;
глобальная конфигурация и ключи не менялись. Первая прежняя попытка Claude с
длинным audit prompt/default effort имела timeout120s; это историческая запись,
не текущий статус обоих коротких smoke.

| Worker | Изолированная ветка/worktree | Первый checkpoint |
|---|---|---|
| codex-z / glm-5.3 | spike/autosiril-render-z; /tmp/minz-autosiril-render-z | Nanz row kernel + raw/final MIR2/native Z80 против203-case Ruby corpus |
| claude-z / glm-5.3[1m] | spike/autosiril-vt2-z; /tmp/minz-autosiril-vt2-z | Actual Ruby Step9..11 vs independent host model и bounded pattern/VT2 design |

Первый checkpoint не заменяет полный objective. После review нужны actual
file stages, весь admitted profile/IO/negative contracts, inherited gates,
required fresh/frozen CI, downloaded artifact audit и own PR/main integration.
Полный VT2 writer отделён от будущего VTX export; не называть planning/kernel
proof готовым native MIDI converter.

Рабочие задания и launcher: /tmp/minz-z-workers/{render-task.md,vt2-task.md,
run_worker.py}. Status/log/final files там же; output roots render-results и
vt2-results. Состояние live проверять по process/session, не только JSON-файлу.
Каждый worker должен оставить WORKER-REPORT.md и локальный commit или diff.
Parent проверяет semantics/coverage/scope и только затем push/PR/merge.

MCP/browser integrations отключены. У workers нет поручения push/merge/release,
сообщения другим, SSH/i7/Herdr, изменение credentials/global config, root user
files, books/workflow или compiler defaults. Shared MIR/pipeline bug сначала
сообщить parent для координации с Claude u7; не исправлять core молча.

Сами runners имеют ограничение30min на checkpoint и сохраняют журналы. При
timeout не перезапускать автоматически: проверить процесс, diff и причину,
передать адресный follow-up. При stop Alice остановить оба процесса; не
продолжать из старого wakeup.

## Проверенный kernel и второй раунд

Kernel checkpoint `215dabd7f617c1f79c6df0dcb0a8fbf79676d3cc` опубликован в
`spike/autosiril-render-z`, интегрирован в parent ветку как `bdfe28bfe`.
Parent заново получил actual-Ruby reference, пересобрал target и сравнил все
609 engine runs и каждый post-row state: 203 cases × raw/final MIR2/Z80,
10320 states на engine. Образ7839B; binary SHA256
`e8158b59563f3357878eb4b13fbe268c4890eaa390f9a5d7e64528d8857c72a0`.
Portable audit: `evidence/parent-kernel-audit.json`. Это kernel proof,
а не самостоятельная шестая файловая стадия.

VT2 worker оставил локальный `3073d8fd7` (554 host cases); parent ещё не
принял его bounded-memory claims. Найдены whole-pattern allocations,
необоснованный u16 order bound и противоречивые размеры transport. Второй
раунд поручен тому же claude-z: streaming spool/index/order, u32 indices,
65535/65536/65567 edges, forced collision, actual ornament provenance и
исправленные измерения. Target VT2 пока не реализован.

codex-z во втором раунде расширяет trace row до u32, проверяет старшие
временные границы и защитные mutants, затем переходит к CP/M file adapter
при наличии времени. Никакого сокращения admitted kind5 profile ради TRD.

Задания и logs первого раунда сохранены с суффиксом `.checkpoint1` в
`/tmp/minz-z-workers/`. Второй раунд запущен тем же runner; актуальную
жизнь процесса проверять отдельно. Root user files повторно сверены:
все четыре SHA256 совпали с baseline; root не изменялся.

## Независимые проверки и найденное end-to-end расхождение

Renderer edge checkpoint `c568353f6` (+ whitespace-only `e805331ae`)
опубликован и интегрирован в parent `3272be35d`. Parent повторил actual Ruby
и все12 positive engine runs: 4 cases,262174 row states на каждый engine.
Есть4 malformed-feed rejects и4 mutants; текущий harness останавливается на
raw MIR2, поэтому это ещё не native-Z80 mutant proof. См. parent-edges-audit.

VT2 checkpoint `2ba66d457` опубликован в `spike/autosiril-vt2-z`, все три
worker commits интегрированы; parent audit `3b45fe960`. Independently replayed
558 cases +548 forced collisions +3 canonical variants,267511 pattern slots.
Output теперь streams header/order/ornaments/patterns до сбора proof statistics.
Это HOST model, не native writer. Audits и corrected source/design сохранены
в `research/autosiril-native/vt2/`.

Обнаружен реальный end-to-end gap: при одинаковых canonical settings Chronos
native kind5 имеет62 отличающиеся p-channel cells, первая row259 note51/orn4
вместо actual-Ruby note58/orn0. Renderer правильно воспроизводит свой вход;
это не доказывает верность всех предыдущих стадий. Parent независимо запустил
chronos_divergence_repro.rb и подтвердил62/row259. Новый isolated worktree
`/tmp/minz-autosiril-p-channel-z`, branch `fix/autosiril-p-channel` от main9cf3a71d5,
передан claude-z для поиска FIRST bad stage и исправления с проверкой настоящих
stage boundaries. Нельзя менять исходный Ruby/настройки, чтобы скрыть разницу.
Exact retained40626B path/inherited gates остаются требованиями; конфликт с
необходимым fix надо явно разобрать. Не заявлять end-to-end equivalence до fix.

codex-z продолжает CP/M adapter в render-z. Его предыдущий30min runner завершился
по timeout после успешного edge proof; состояние проверено, новый task адресный,
не рестарт из-за одного observation timeout. claude-z после завершения VT2
переключён на upstream investigation; runner/task/logs архивированы `.checkpoint3`.
