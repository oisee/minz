# PFCCO доступен обычной компиляции и CLI

`Options{ContractOpt:true, ContractStrategy:"pbqp"}` теперь выбирает совместные контракты PFCCO в обычных `CompileHIRSteps` и `CompileHIRWithOptions` **до LUTGen, PreallocCoalesce и PBQPAllocate**. CLI: `--contract-strategy pbqp`. Пустая стратегия сохраняет прежний greedy; явное `greedy` тоже разрешено. Greedy optimizer не изменён.

```sh
cd minzc
go build -o /tmp/mz ./cmd/minzc
/tmp/mz ../examples/nanz/pfcco_weighted_packet.nanz --contract-strategy pbqp --asserts none -o /tmp/packet.a80
/tmp/mz /tmp/packet.a80 -o /tmp/packet.bin
```

Неизвестная стратегия, явная стратегия при выключенном ContractOpt, а также PBQP с eZ80/другим target backend отклоняются. Проверка общая для pipeline и `ProfileCompilationID`. CLI не позволяет явную стратегию сочетать с profile search/collection/replay: существующие четыре режима остаются историческими greedy/off, пространство поиска не расширено. Ordinary API evidence ID автоматически включает непустое поле JSON; `omitempty` оставляет прежний JSON/ID пустой стратегии. Старый профиль при смене стратегии отказывается применяться без изменения HIR. Старые evidence/artifact IDs не переписывались.

## Реальный исходник и native результат

Новый Nanz test содержит `twice(@z80_c x)` с настоящим inline ASM marker и неизменяемым C ABI, свободную `offset(x)=2*x+3`, вызывающую `multi(x)`. Обе функции изменяют byte global trace. Второй CALL реально выполняется только при x!=0. На каждом x=0..255 независимая арифметика u8, MIR VM и полный assembler/native Z80 согласны; global state переносится между вызовами, SP восстановлен. Машинное измерение включает все реальные CALL/RET, переносы и сохранения регистров.

| Путь | Greedy | PFCCO PBQP |
|---|---:|---:|
| pinned исходник, все 256 входов | 87 387 T / 63 B | 85 343 T / 61 B |
| неизменённый weighted packet, 17 вызовов | 7 888 T / 52 B | 7 888 T / 52 B |
| weighted packet без ContractOpt | 8 857 T / 52 B | — |

Weighted packet test мигрирован с ручного post-allocation переопределения на реальный pipeline option. Для него PBQP не даёт дополнительного выигрыша относительно greedy. Сохраняются два реальные вызова helper, независимый u16 displacement oracle и глобальное состояние.

Польза локальна; модель PFCCO всё ещё эвристическая, residual `3+x` из отчёта 056 остаётся. Ни общий corpus успех, ни native оптимальность не заявляются.

## Проверки

Активны default/explicit greedy equivalence, unknown/disabled/eZ80 rejection, разные config IDs, stale profile strategy rejection без HIR mutation, CLI search/replay refusal, native pinned source и packet. Реальный собранный CLI сгенерировал ASM и BIN 52 B и отказал несовместимым флагам. Focused pipeline/compat/replay tests PASS; `go test ./pkg/... ./cmd/... -run '^$'` PASS. Broad `go test ./... -run '^$'` упирается в прежний scripts mixed-package/relative-import layout, не в production пакеты.

Артефакты и SHA256: `reports/grace-pfcco-strategy-artifacts/`.

Полный CLI на базе 0be9ab59: FAIL 117.500 s, единственное падение `TestSearchCLICollectAndPairedReplay` (exact original SelfLanz source required). Независимый pristine 0be9ab59 воспроизводит его за 0.089 s; parent уже исправил historical fixture в c784dc4a. Этот коммит не изменяет search trust или fixture.
