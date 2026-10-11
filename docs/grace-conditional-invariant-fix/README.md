# Conditional invariant: исправлен native miscompile при исчерпании scratch registers

База `fed26a86`. Повторены падения полного pipeline из checkpoint `pipeline-cb587-full.txt` на текущем коде: four-parameter mask1/3 возвращали **257 вместо 1**, six-parameter mask0/2 — **513 вместо 1**, при MIRGrace on/off. Падали и настоящие site candidate selectors. `before.txt` сохраняет текущий воспроизводимый провал, не только старый checkpoint.

## Причина и исправление

Для four-mask1 invariant `(65535+2)` правильно вычислялся как u16=1 в DE. Все byte registers в loop header заняты живыми параметрами/парами/counters. Перед сравнением код сохранял pending A (границу n=1) через legacy fallback:

```asm
LD D,A    ; materialize pendingAcc
```

Эта запись заменяла старший байт живого DE: `0x0001` превращалось в `0x0101=257`. В six fixture n попадал в аналогичный живой байт и давал 513. HIR rewrite и raw/final VM сохраняли правильную арифметику; проблема в production Z80 materialization.

`materializePendingAcc` теперь использует `freeScratch8`; при отсутствии свободного байта сохраняет A в **именованный byte spill** через `emitLD8`, затем existing physOverride/terminator restoration возвращают его. Живые DE/HL/BC/IX/IY не заимствуются. Это исправление фактически воспроизведённого materialization path; другие legacy scratch consumers не переписаны без отдельного evidence.

Для failing fixture теперь:

```asm
LD (_spill_scalar_r10),A
...
LD A,(_spill_scalar_r10)
```

Byte spill может увеличивать размер и T-states в случаях высокого register pressure. Это оправдано корректностью, а выбор оптимизации измеряет получившийся реальный код. Никакие кандидаты, маски или исходные ожидания не исключены.

## Независимое доказательство

Исходные fixture algorithms сохранены. Model по Language Book v7 §3.2 вычисляет u16 sum modulo65536 независимо от HIR/ASM. Расширение workload: 6 разных u16 пар (carry, zero, equal high bits, 255+1,65535+65535), 6 trip counts 0/1/2/5/127/255 и flags0/1 — **72 вызова** каждого варианта.

- 2 исходных формы (four/six), 4 mask и MIRGrace off/on: **1152 native вызова**, отдельно raw VM и final production MIR проверены против модели.
- `TestConditionalInvariantSiteCandidates`: все четыре кандидата с полной нагрузкой.
- `TestInvariantSiteRulesConditionalCandidates`: обе формы и все восемь rule/mask candidates; никакой частичный winner/фильтр падений.

Workload вырос в три раза; cumulative MaxSteps увеличен 200000→1000000 для выполнения полной расширенной нагрузки. Арифметические ожидания не изменены. ASM/BIN/HIR/raw/optimized MIR и oracle workload/expected сохранены для всех 16 regimes.

Focused three-family tests PASS (`after.txt`). Полный MIR2 package PASS **13.118 s** (`mir2-package.txt`). Полный медленный pipeline не повторялся этим этапом; parent интегрирует и проверяет общие gates.

## Воспроизведение

Из `minzc/`, Go1.26.8:

```sh
MINZ_CONDITIONAL_ARTIFACTS=/absolute/evidence go test ./pkg/pipeline -run '^TestConditionalInvariantRegression$|^TestConditionalInvariantSiteCandidates$|^TestInvariantSiteRulesConditionalCandidates$' -count=1 -v
go test ./pkg/mir2 -count=1
```

`before/four/MIRGrace=false/mask1/` сохраняет конкретный old ASM/BIN. `after/` хранит все 16 сборок. JSON и MIR gzip без потери; manifest содержит SHA256 всех payloads кроме себя.
