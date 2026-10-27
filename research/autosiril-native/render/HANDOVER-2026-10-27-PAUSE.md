# HANDOVER: пауза native Autosiril по команде Alice, 27 октября 2026

Этот документ — единственный актуальный handover на паузе. Предыдущий
`render/HANDOVER-2026-10-27.md` описывал более раннюю паузу (после релиза
книги) и частично устарел; при конфликтах верить этому файлу и
`/tmp/minz-z-workers/HANDOVER-INVENTORY-2026-10-27.json`.

## 1. Почему остановились и главное правило

Alice явно попросила **STOP реализации на weekly-лимите 4%** и поручила через
docs-воркеров только: handover, статусы, commit/push сохранённого, статью.
Файл-маркер: `/tmp/minz-z-workers/USER-STOP-2026-10-27.txt`. Legacy STOP marker
`~/.local/share/minz-crystal-hir-wakeup/STOP` на месте, не удалять.

**Ничего не возобновлять (воркеров, goals, CI-правки, merges, релизы) без
следующей явной команды Alice.** Книга и релизы уже сделаны — НЕ пересобирать:
`book-compiler-days-2026-10-27` (main `9cf3a71d5`, PR271), первая редакция
`book-compiler-day-2026-10-26` нетронута.

## 2. Живое vs остановленное (проверено по процессам, не по JSON)

Проверено `ps` на момент написания (≈16:45 2026-10-27):

- **Остановлены** (PID из status-файлов отсутствуют в процессе):
  - `codex-z` (CP/M renderer, pid 11838) — completed/returncode −15, остановлен по STOP;
  - `claude-z` (canonical6 TRD, pid 9987) — returncode 143 (SIGTERM), 835.6 c;
  - `claude-vt2` (native VT2, pid 7180) — timeout 1800.2 c, returncode 143.
- **Во время подготовки были живы docs-воркеры**: `docs-handover` (pid 13384) и `docs-article`
  (pid 13400) + их обёртки `run_worker.py` (13395/13364). Они пишут только
  этот файл и `reports/autosiril-native-2026-10-27.html`, затем завершатся.

Правило сохраняется: жизненность любого воркера проверять по процессу/сессии,
status-JSON — только подсказка.

## 3. Worktrees, ветки, HEAD; закоммиченное/запушенное vs WIP

Живой git на момент handover (authoritative; inventory-JSON писался чуть раньше
и для двух worktrees уже отстает — см. примечания):

| Worktree | Ветка | HEAD | Push? | WIP |
|---|---|---|---|---|
| `/tmp/minz-autosiril-native-render` (parent) | `spike/autosiril-native-render` | `bbbf66853` (код до финального docs-коммита) | да | документы сохраняются отдельным финальным коммитом |
| `/tmp/minz-autosiril-render-z` | `spike/autosiril-render-z` | `6bbda83ae` (native: prove saved-file CP/M cases + admission gaps) | да, 0/0 | чисто |
| `/tmp/minz-autosiril-p-channel-z` | `fix/autosiril-p-channel` (= **PR272**) | `55af8f924` | да, 0/0 | чисто |
| `/tmp/minz-autosiril-canonical6-trd-z` | `spike/autosiril-canonical6-trd` | `4cf27eb29` `wip: preserve canonical6 TRD proof script at user pause` | да, проверено `git ls-remote` | чисто |
| `/tmp/minz-autosiril-vt2-native-z` | `spike/autosiril-vt2-native-z` | `13420ca32` `wip: preserve native VT2 kernel draft at user pause` | да, проверено `git ls-remote` | чисто |
| `/tmp/minz-autosiril-vt2-z` (host VT2, закрыто ранее) | `spike/autosiril-vt2-z` | `2ba66d457` | да | — |
| `/Users/alice/dev/minz` | `main` | `9cf3a71d5` | — | пользовательские файлы, см. §10 |

Примечания:

- Два `wip:`-коммита сохранения сделаны parent'ом **после** снапшота
  inventory-JSON (там головы `55af8f924`/`bbbf66853`) — верить git. Оба WIP
  содержат собственный `PAUSE-STATUS.md` с честным статусом «не проверено».
  Оба preservation-коммита уже запушены; remote SHA проверены parent через
  `git ls-remote`, **без** заявления об исполнении.
- В `render-z` незакрытый CP/M adapter ранее был сохранён отдельным коммитом
  `8316d9944`, поверх — доказанный checkpoint5 `6bbda83ae`.
- Ветки `canonical6-trd` и `vt2-native-z` ответвлены от `55af8f924` /
  `bbbf66853` соответственно; PR272-ветку не трогать, пока CI не разобран.

## 4. PR272 и CI — статус на паузе

- PR272 = `fix/autosiril-p-channel` (head `55af8f924`), «canonical6 file chain +
  fresh/frozen CI jobs». **CI run 40540793356 был IN PROGRESS на момент
  паузы. MERGE НЕ ДЕЛАЛСЯ и запрещён до полной проверки.**
- Порядок после команды Alice: дождаться required green (включая новые
  `native_echo_fresh` canonical6-fresh и `native_mix_frozen` canonical6-frozen
  артефакты `autosiril-canonical6-fresh/-frozen`), скачать артефакты заново,
  сверить SHA256/bytes, прогнать независимый аудит, только потом merge через
  GitHub с expected head SHA и fast-forward main. Никаких merge «по зелёному
  бейджу» без download-audit.
- Ссылка: https://github.com/oisee/minz/pull/272 ; CI:
  https://github.com/oisee/minz/actions/runs/40540793356
- Существующие audits-скрипты-образцы (не переносимы, /tmp):
  `/tmp/minz-mix-artifact-audit.py`, `/tmp/minz-echo-artifact-audit.py`;
  скачанные артефакты — `/tmp/minz-ci270-artifacts/`.

## 5. Что реально доказано к паузе (точный охват)

Общая база (сделано ранее, не трогать): Crystal→HIR native semantic export
(inference/специализации/merge), macro↔meta, ByteArena 7B target, marker
offsets target vs host, scalar MIR2→Go, each→foreach, generators/pull
(break/next/nonlocal, RLE, demand), общая byte-библиотека (538 наборов),
ring FIFO + два borrowed segments (525 наборов), generator frame liveness
(53→13B, 6→3B). Детали и цифры — `research/crystal-hir/ITERATION*.md`,
таблица в старом `HANDOVER-2026-10-27.md`.

Native файловый pipeline (пять независимо линкуемых CP/M стадий
MIDI→PAIR→NOTES→FLATTEN→EVENTS→ARRANGE→ECHO→MIX, PR270) + retained
**exact 40626B** in-process путь (SHA256 `bc440efa…`) сохранены; все прежние
gates 1923 исполнений + 6 native guard mutants на gate.

Седьмая (renderer) работа этого дня:

1. **Row kernel** (`render/kernel.nanz`): 203 случая actual-Ruby corpus ×
   {raw MIR2, final MIR2, assembled Z80} = **609 runs**, 10320 post-row
   states на engine; image 7839B, binary SHA256 `e8158b59…`. Это kernel с
   fixture traps (RST 8 / RST 0x10), не файловая программа.
2. **High rows / feed guards**: 4 случая (65535, 65536, 65567, empty 65536) —
   **262174 row states на каждый engine**; feed-guard faults 958/958/904/983;
   4 защитных мутанта — пока **raw-MIR2 negative controls** (harness
   останавливается на первом raw failure), native-Z80 mutant coverage —
   будущий gate. Wire: 77B packet u16 row ≤65535, 79B u32 сверху.
3. **Host VT2** (закрытый этап, отдельная ветка): 558 cases + 548 forced
   collisions + 3 canonical variants, 267511 pattern slots, streaming
   header/order/ornaments/patterns. **HOST model, не native writer.**
4. **CP/M renderer adapter** (checkpoint5, `6bbda83ae`): parent независимо
   повторил — **114 исполнений** (25 сохранённых native kind5 входов ×3 engine
   + 13 wire/BDOS guards ×3), Chronos exact 67600 ASCII bytes; определены
   STATE.BIN v1 / ARMD META.BIN v1 / REND.INF v1. **Полный admitted profile
   НЕ закрыт** — см. §6.
5. **Canonical6 (max_offset 6) CP/M file chain** (PR272): actual
   ARRANGE-CANONICAL6.COM (13646B) → неизменный ECHO.COM → **actual MIX.COM**
   через mze; 0 diff на всех границах против actual Ruby (2097 groups/cells,
   3215 cells, render_rows 1352, MIXED@6 logical 54880/physical 54912B,
   sha256 `8aa9cfa2…`); стадии 1/2 переиспользованы как byte-identical
   verified upstream (не реисполнены); плюс fresh (с Ruby) и frozen (без Ruby,
   `replay_canonical6.py` + `fixtures/canonical6-frozen.json.gz`) proofs.
   Parent независимо повторил оба — zero field/missing/extra diffs (коммиты
   `b9eec10b3`, `40dc4c4a3`).
6. **Причина «62 разных p-channel cells» закрыта**: это был
   **конфигурационный mismatch max_offset 24 (native standalone) vs 6
   (canonical Ruby argv)**, НЕ codegen bug. Flatten совпадал (2097 groups),
   расхождение росло 31→62→62. Legacy-профиль 24 легитимен и сохранён со
   всеми goldens; canonical6 — отдельный явный профиль.

## 6. Незакрытое WIP — точное состояние на паузу

### CP/M renderer: 5 дыр admission-профиля (главный приоритет)

Полный актуальный список с деталями — **`/tmp/minz-z-workers/render-task.md`
(пункт 1–5), это канонический текст задачи**. Кратко:

1. **Definitions profile**: правильный контракт — 64 **семантических**
   definitions, aggregate payload 4096B, фрагмент = max 8 payload + 8 header
   bytes. Текущий cache `definition_payloads` 4096/256 записей может
   **исключать легальные длинные определения**; worst fragment count >512
   (каждое определение округляется). Различать **distinct definition count vs
   raw fragment record count**; ссылка проверяется против fragments — может
   принять undefined id; cursor инкрементируется на фрагмент, не на
   определение. `ruby_case` assert `len(definitions)<=64` сейчас считает
   сырые фрагменты и режет легальный вход. Не сужать профиль.
2. Kind5 wire порядок — **definitions→cells→4 metadata→END** (гипотеза
   «metadata first» была неверна и исправлена по byte-level second opinion).
3. **Cached ENOTE bytes 13..14**: adapter пересчитывает cache по kernel, но не
   валидирует когерентность с source; malformed kind5 нельзя молча
   нормализовывать. Добавить target-guard на corrupt cached field и
   semantic/reserved bytes.
4. **REND.INF = 17 логических bytes** (RINF4 + ver1 + 3×u32), а не 21 как в
   docs/evidence. Исправить контракт до 17, аудитор должен проверять padding
   от 17.
5. **Canonical6 вход для renderer**: actual
   `/tmp/minz-z-workers/p-channel-results/parent-p3-fresh/standalone6-mix/{MIXED,DELAYED,NOTES}.BIN`,
   затем независимо слинкованный CP/M renderer COM и six-COM цепочка через
  `mze` (`/Users/alice/.local/bin/mze` работает на mac; чистая сборка на mac
   падает на PREEXISTING syscall.Select — root-файл с фиксом пользователя
   НЕ трогать; Linux CI собирает из исходника в PR272).

См. также `/tmp/minz-z-workers/render-feedback.md` (META.BIN — не валидный
kind5, а metadata-компаньон: нужны discriminator/отдельный контракт и
roundtrip-проверки; доказывать empty/one-cell раньше Chronos) и
`/tmp/minz-z-workers/cpm-second-opinion.md` (независимые byte-order/framing
находки; snapshot-специфичны — сверить hash с текущим кодом).

### TRD canonical6: скрипт сохранён, НЕ исполнен

`4cf27eb29` сохраняет `p-channel-fix/trd6/prove_trd6.py` (433 строки,
`PAUSE-STATUS.md` честно говорит: unfinished/unverified). **Прохождения
нет, не изобретать pass.** Задача —
`/tmp/minz-z-workers/canonical6-trd-task.md`: canonical6 через существующие
TRD arrange→echo→mix target-стадии (замена `max_offset 24→6` только в
GENERATED source), все 3 engine где драйверы позволяют, сравнение kind3/4/5
файлов с frozen canonical6 ожиданиями. Профили/капы 9216 cells/64 defs/4096B
и все 16-bank guards не ослаблять. Plain render 67600B > TRD 65280 —
будущий TRD renderer обязан сегментировать или честно фейлить, не резать.

### Native VT2: kernel draft, НЕ исполнен

`13420ca32` сохраняет `vt2/native/{kernel.nanz 625 строк, kernel-driver.go.txt,
prove_native.py, PAUSE-STATUS.md}`. Первый checkpoint **timed out на
починке Go emulator/API**; ни одного raw/final/native прогона нет. Продолжение
(первое действие, не «20 минут исследования парсера»): взять ГОВОРИВШИЕ Go
API из работающего render-харнесса (см. `render-feedback.md`: proven
`kernel.nanz` extern8/16 ABI, bounded global mailbox + `@z80_c/@z80_de`
pointer trap; host — только byte IO), скомпилировать driver, прогнать один
маленький случай против unchanged Ruby, потом canonical/negative/upper-bound.
Задача: `/tmp/minz-z-workers/vt2-native-task.md`; требования: target считает
все offsets/hash/dedup/order/ASCII, u32 индексы (render_rows 65567 ⇒ 65568
slots), обязательный forced-collision digest test, без whole-header RAM.
VTX packing — отдельная будущая работа.

## 7. Открытые вопросы (из authoritative inventory)

- CPM: distinct defs vs raw fragments, 4096B payload vs 256 raw-записей,
  decoder raw def ≤64; профиль не сужать.
- Cached ENOTE когерентность/undefined refs — валидация.
- RINF фактически 17 logical bytes, docs говорили 21.
- canonical6 + six COM renderer input — pending.
- TRD canonical6: скрипт начат, verified completion нет.
- native VT2: kernel/prover/driver написаны, runner 30min timeout,
  target execution proof нет.
- PR272 CI 40540793356 — дождаться required green + downloaded artifact audit,
  merge только после.

## 8. Критические уроки (не наступить повторно)

1. **canonical6 = max_offset 6, legacy = 24.** Оба профиля легитимны; прежние
   62 различающиеся клетки — разница конфигурации, не codegen bug.
2. **Kind5 порядок: definitions→cells→4 metadata→END.** «Metadata first»
   было ошибкой; правильно по byte-level второму мнению.
3. **Семантические definitions ≠ сырые фрагменты.** Полный 4096B payload
   profile ещё не доказан; не подменять одно другим в проверках и отчётах.
4. **Не выдумывать boundedness**: whole header/order strings в RAM — не
   bounded; реально — file-backed spool/index/order, chunk compares.
5. **Мутанты обязаны реально исполнять ветку и на ВСЕХ engine**; harness,
   останавливающийся на первом raw-MIR2 failure, даёт только raw negative
   control, не native mutant proof.
6. **Trace row — u32, source row kind5 — u16** (≤65535). Двойной wire: 77B
   u16 packet / 79B u32 packet.
7. **Ширина текста 46..49 chars** (не всегда 49; sample 32/255 даёт nil
   character); Chronos plain text **67600B > TRD 65280B** — нужен
   сегментированный/lossless дизайн или честная size failure, не обрезка.
8. **Различать уровни заявления**: host model ≠ target kernel ≠ file-linked
   stage ≠ physical hardware. API traps/Scorpion TRD sector API — не ROM/FDC/
   boot/TPA; эмуляторные T-states — не realtime.
9. **Guards**: физическое padding/выравнивание (REND.INF от 17), cleanup/
   publication семантика (partial после catalog write — проверено, не
   атомарно), owned cursors per reader (не shared cursor_record_count).
10. Fresh ≠ frozen: fresh пересобирает reference с Ruby, frozen переиспользует
    замороженные ожидания, но target исполняется заново; downloaded artifacts
    проверять отдельно от бейджа.

## 9. Приоритеты и команды возобновления (только после команды Alice)

Лучший порядок: **(1) CP/M полный profile, (2) PR272 CI разбор, (3) TRD
canonical6, (4) native VT2, (5) six-COM цепочка + финальные CI/audits**.

### CP/M (render-z, ветка `spike/autosiril-render-z`)

```sh
PYTHONDONTWRITEBYTECODE=1 python3 research/autosiril-native/render/prove_cpm.py \
  --artifacts /tmp/minz-ci270-artifacts/autosiril-mix-fresh \
  --autosiril /Users/alice/dev/autosiril \
  --planning /tmp/minz-render-planning-v4 \
  --out /tmp/minz-z-workers/render-results/checkpoint6-cpm-a
```

Эталон прошлого прогона: `/tmp/minz-z-workers/render-results/checkpoint5-cpm-n/measured/receipt.json`,
evidence `research/autosiril-native/render/evidence/{cpm-checkpoint5.json,parent-cpm-checkpoint5-audit.json}`.
Дальше — пункты 1–5 из `render-task.md` (§6 выше), затем canonical6 вход
из `parent-p3-fresh/standalone6-mix/`.

### canonical6 fresh/frozen (p-channel-z, только чтение/повтор)

```sh
python3 research/autosiril-native/p-channel-fix/prove_canonical6.py \
  --autosiril /Users/alice/dev/autosiril \
  --upstream /tmp/minz-ci270-artifacts/autosiril-mix-fresh/upstream/cpm/standalone \
  --mix-com /tmp/minz-ci270-artifacts/autosiril-mix-fresh/cpm/standalone/MIX.COM \
  --mze /Users/alice/.local/bin/mze \
  --fixture research/autosiril-native/p-channel-fix/fixtures/canonical6-frozen.json.gz \
  --out /tmp/p3-fresh

python3 research/autosiril-native/p-channel-fix/replay_canonical6.py \
  --upstream /tmp/minz-ci270-artifacts/autosiril-echo/cpm/standalone \
  --mix-com /tmp/minz-ci270-artifacts/autosiril-mix-frozen/cpm/standalone/MIX.COM \
  --mze /Users/alice/.local/bin/mze --out /tmp/p3-frozen
```

Эталоны: `/tmp/minz-z-workers/p-channel-results/{parent-p3-fresh,parent-p3-frozen}`
(+`.log`). Прошлые parent-прогоны — коммиты `b9eec10b3`/`40dc4c4a3` в
`spike/autosiril-native-render`.

### PR272 CI

```sh
gh pr view 272 --repo oisee/minz --json state,mergeable,statusCheckRollup
gh run view 40540793356 --repo oisee/minz
gh run download 40540793356 --repo oisee/minz --dir /tmp/minz-pr272-artifacts
```

Затем независимый аудит скачанных fresh/frozen артефактов (SHA256/bytes против
GitHub), и только при полном required green — merge с expected head SHA
`55af8f924` + fast-forward main с сохранением root пользовательских файлов.

### TRD canonical6 / native VT2

Начать с чтения `trd6/PAUSE-STATUS.md` + `prove_trd6.py` и
`vt2/native/PAUSE-STATUS.md` соответственно (см. §6). Сначала запушить
preservation-коммиты `4cf27eb29`/`13420ca32` (без заявления об исполнении).

## 10. Неизменные ограничения и пины

- **Оригинальный Ruby**: `/Users/alice/dev/autosiril` — только читать. Пины:
  repo commit `44925349147d5cd4bf3636c3f0eeffa7b46e261c`, `autooisee.rb`
  SHA256 `41f621b555fc089effd711319e723494576eb47ea0282847082ec91c1cdd4c01`.
- **Root `/Users/alice/dev/minz`**: не вести там новый код. Четыре
  пользовательских файла сохранять и не stash/reset/add
  (`minzc/cmd/mze/cpm_tick_input.go` — там незакоммиченный пользовательский
  фикс Select-signature, НЕ коммитить; три `reports/crystal-*-2026-10-25.html`;
  контрольные SHA256 — в старом handover). При расхождении — разбираться,
  не откатывать.
- **i7 — только чтение**, ничего тяжёлого. **Herdr без HERDR_ENV=1 не
  трогать**, не запускать bare herdr, не подделывать env, не возобновлять
  старую будилку; STOP marker не удалять.
- Один merge за раз, только green CI; shared MIR/pipeline/codegen изменения —
  сначала координация с Claude (u7).
- Ключи/credentials не выводить и не включать в prompts/документы (см. §11).

## 11. Оркестрация GLM-воркеров (как это запускалось)

- Обёртки в `~/.zshrc`: **`claude-z`** (Claude Code через
  `https://api.z.ai/api/anthropic`, модели → `glm-5.3[1m]` /
  `glm-5.3-flash[1m]`, auth `ZAI_API_KEY`) и **`codex-z`**
  (`codex -c model_provider=zai -m glm-5.3`, provider
  `https://api.z.ai/api/v1`, `wire_api=responses`). Провайдер — GLM от Z.ai.
  Ключи не показывать и не копировать в документы.
- Раннер: `/tmp/minz-z-workers/run_worker.py <name>` — 30-минутный checkpoint
  timeout (`SIGTERM`→`SIGKILL` по группе), пишет `<name>-status.json`,
  `<name>.log`, `<name>.stderr`, финал в `codex-z-final.txt`. Воркеры только
  read-only относительно чужих worktree; без push/PR/merge/SSH/keys.
- Задачи/логи/архивы: `render-task.md*`, `vt2-task.md*`,
  `vt2-native-task.md`, `canonical6-trd-task.md`, `p-channel-task.md*`,
  `handover-task.md`, `article-task.md`; логи `claude-z.log*`, `codex-z.log*`,
  `claude-vt2.log`; статусы `*-status.json*` (архивы `.checkpoint1..5`,
  `.checkpoint-p1..p3`). Feedback-файлы parent→воркер:
  `render-feedback.md`, `p-channel-feedback.md`, `vt2-native-feedback.md`,
  `cpm-second-opinion.md` (читать перед следующим фиксом).
- Выходные корни: `render-results/`, `vt2-results/`,
  `vt2-native-results/`, `canonical6-trd-results/`, `p-channel-results/`;
  снапшот для ревью `cpm-review-snapshot/`.
- Жизненность проверять по процессу; при timeout не перезапускать вслепую —
  сначала diff/лог/причина и адресный task. При STOP Alice — остановить все
  процессы; сейчас они уже остановлены (§2).
- `/tmp` непersistентен: durable proofs — только в repo/evidence; всё
  ценное из results-каталогов при возобновлении перенести в ветки.

## 12. Что заявлено, а что нет (итоговая рамка)

Заявлено и независимо проверено: row kernel 609 runs, edges 262174
row-states/engine, host VT2 corpus, CP/M 114 исполнений на legacy-профиле,
canonical6 fresh+frozen CP/M file chain (0 diff на всех границах), книга и
релизы. НЕ заявлено: полный CP/M admitted profile (64 defs/4096B payload,
ENOTE cache, RINF 17), TRD canonical6 (скрипт WIP), native VT2 (kernel WIP,
0 исполнений), six-COM каноническая цепочка, VT2/VTX writer, physical
hardware. PR272 не смержен. Возобновление — только по явной команде Alice.

## Финальная сверка parent после документационных воркеров

Оба docs-воркера завершились; implementation goal остаётся `paused`.
Remote refs проверены: render `6bbda83ae`, canonical6 PR `55af8f924`,
TRD WIP `4cf27eb29`, native VT2 WIP `13420ca32` — всё запушено.
Оригинальные четыре root user files сохранены. Handover, статья и
машиночитаемый `PAUSE-2026-10-27.json` публикуются финальным docs-коммитом
в `spike/autosiril-native-render`. CI #40540793356 при последней сверке
ещё выполнял основные compiler/frozen и fresh jobs; остальные наблюдаемые
checks были зелёными. PR #272 не смержен, новые реализации не запускать
без следующей команды Alice. Статья: `reports/autosiril-native-2026-10-27.html`.
