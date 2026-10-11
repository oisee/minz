# Обновлённый корпус: выбор зависит от сборки

После direct global u16-load backend те же четыре persistent workloads
измерены заново на сохранённой production CLI dba34. Исходник и входные
workload файлы совпадают с прежними сохранёнными CLI inputs байт в байт.
Четыре набора содержат50 calls; восемь CLI runs измерили32actual кандидата.
Это artifact-only этап, compiler и runtime не менялись.

| Набор | Original T | Arena T | Append T | Both T |
| --- | ---: | ---: | ---: | ---: |
| Short reuse | 330515 | 331075 | 327008 | 327568 |
| Long atoms | 4110605 | 4110877 | 3968942 | 3969214 |
| Nested mixed | 1297899 | 1299550 | 1281762 | 1283413 |
| Fresh dominant | 789462 | 789510 | 699845 | 699893 |

Все четыре набора выбирают **append-only для speed и size**. Старая сборка
1bd выбирала both для speed. Arena на новой сборке добавляет560/272/1651/48 T
к append соответственно; выигрыш append относительно текущего original —
**1.06/3.45/1.24/11.35%**. Размеры четырёх кандидатов2000/2121/1978/2099 B,
выбранного1978 B. Это observed crossover на этих workloads, не универсальный
winner. На другом full30 workload новаяCLI по-прежнему выбирает both для speed.

Два независимых saved-file аудита подтвердили32 native/raw fullstate/results/
ordered byte events,8freshprofiles cumulative/per-call11source counts,
source/unit/workload/currentexe identity, precompile proofs, native EDroots/
sidecars/finiteframes/allocation/MIR hashes и8actual stored decisions/selected
ASM/BIN. Старый1bd и новыйdba контексты раздельны; профиль не переподписывали.

Пакет сохранён локально, включая40 BIN,8executionlogs,161 deterministicgzip
payload с SHA исходных bytes. После переноса исправлен поиск `.gz` в
portable audit helper; полный аудит файлов повторён без CPU и прошёл.

[Пакет](grace-global-word-load-corpus-artifacts/refresh/README.md) ·
[Независимый аудит](grace-global-word-load-corpus-artifacts/independent-oracle/FINAL-AUDIT.md) ·
[Native review](grace-global-word-load-corpus-artifacts/native-review/FINAL_REVIEW.md) ·
[SHA256 manifest](grace-global-word-load-corpus-artifacts/manifest.json).

```sh
python3 -B docs/grace-global-word-load-corpus-artifacts/independent-oracle/audit_refresh.py docs/grace-global-word-load-corpus-artifacts/refresh/evidence --source docs/grace-global-word-load-corpus-artifacts/refresh/source.nanz --out /tmp/corpus-refresh-audit.json
```

Следующий срез — private normal-global diamond load forwarding. Его rawMIR
prototype не включён в production; источник допустимости и native замеры
должны быть проверены отдельно. Общие CSE/InlineTrivial guards не ослабляются.
