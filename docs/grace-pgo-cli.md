# Воспроизводимый PGO через CLI

Один и тот же собранный `mz` собирает controlled VM профиль, измеряет original/pointer на assembled Z80, выбирает вариант и сохраняет решение. SHA256 самого executable входит в compatibility key; после пересборки нужен новый профиль и новая модель.

```sh
mz examples/nanz/01_sum_array.nanz \
  --hir-profile-auto /tmp/sum-pgo \
  --hir-profile-function sum_array \
  --hir-profile-auto-workload docs/grace-pgo-decision-artifacts/workload.json \
  --asserts=none -o /tmp/sum-pgo/selected.bin
```

Directory содержит workload/profile/run/model/evidence/decisions JSON и selected HIR/raw MIR/optimized MIR/ASM. Evidence хранит 1022 реально исполненных address/length сценария, binaries обеих версий и SHA256 execution binaries. Коэффициенты выводятся из measured rows и проверяются на полном домене, а не подставляются по имени функции. Каждый сценарий проверяется независимым sum oracle, raw MIR2 VM и assembled Z80; input bytes не изменяются, guard branches действительно наблюдаются. EvidenceID — SHA256 канонического JSON evidence.

Scope automatic runner: один exact guarded u8 sum site в измеряемой функции; остальные mapped sites остаются original. Параметры pointer/u8, scalar u8 accumulator return. Остальные формы отказываются до выбора. Четыре семейства адресов покрывают fast/equality/first-fallback/wrap, не все 65536 адресов и не все возможные data patterns. Полная программа компилируется; стоимость измеряется от входа в функцию до RET, без caller/bootstrap. Legality остаётся отдельной проверкой Grace.

`--Osize` измеряет модель для своих compilation options и выбирает меньший модуль; прежнюю speed-модель с изменёнными options применять нельзя. Компактный `.bin` использует default assembler и origin 0x8000, совпадающие с measured module binaries. ASM output не содержит startup/entrypoint для самостоятельного запуска целой программы.

Раздельные операции:

```sh
mz examples/nanz/01_sum_array.nanz \
  --hir-profile-collect /tmp/profile.json \
  --hir-profile-workload docs/grace-pgo-decision-artifacts/workload.json \
  --hir-profile-run-result /tmp/run.json --asserts=none

mz examples/nanz/01_sum_array.nanz \
  --hir-profile /tmp/sum-pgo/profile.json \
  --hir-profile-workload /tmp/sum-pgo/workload.json \
  --hir-profile-model /tmp/sum-pgo/model.json \
  --hir-profile-decisions /tmp/reused-decisions.json \
  --asserts=none -o /tmp/reused.bin
```

Раздельная collection не компилирует output. Failed runs сохраняются явно помеченными и loader их отвергает. Unknown fields, повторяющиеся JSON keys (включая различный регистр), trailing JSON и oversized input отвергаются. Внешняя модель в раздельном apply — supplied evidence; loader не является криптографической аттестацией чужого измерителя. Automatic path измеряет здесь и сейчас. Legacy `--pgo` (.tas) — отдельный прежний путь, новые флаги с ним несовместимы.

Следующий этап общей цели: несколько sites/функций и обычные caller workloads, provenance через MIR transforms, сравнение на корпусе и дополнительные guards/certificates.

## Следующий срез: модуль с несколькими функциями

Измерение одной exact sum функции допускает другие mapped loops в модуле и явно оставляет их original. Модель остаётся привязана ко всему typed module. Независимые модели нельзя складывать как доказательство выгодности нескольких одновременных rewrites: PBQP и размер всего модуля зависят от комбинации. Проверка двух sum функций показала +47 bytes для одного rewrite вместо +49 в одиночном примере. Общий эффект на caller workload требует отдельного assembled execution; текущие totals относятся только к измеряемой функции.

`SelectLoopVariantsWithModels` и `ProfileSelection.Models` принимают уникальную модель для каждой покрытой функции. Uncovered sites сохраняют original и пустые totals. Все profiles/models проверяются до изменения HIR; дубликаты, устаревшие IDs/config и более одного pointer выбора отвергаются. Single-model API сохраняет совместимость.

[Joint workload runner](grace-pgo-joint.md) добавляет `--hir-profile-joint` к automatic CLI: до16 комбинаций исполняются целиком с обычными caller calls. Его evidence позволяет выбрать несколько pointer rewrites, поскольку набор измерен совместно; independent-model API остаётся консервативным.
