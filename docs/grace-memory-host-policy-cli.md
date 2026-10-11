# Явная policy в memory-host CLI v2

CLI v2 измеряет четыре arena/append варианта при одной явно выбранной backend
policy и сохраняет выбранные ASM/BIN прямо из проверенного Target record.
Пять actual запусков и два независимых аудита прошли. На полном workload
Speed выбрал both: **1076715 T / 2100 B**; Size выбрал append: **1076874 T / 1979 B**.
CLI перенесён в рабочую ветку `714f9e36`; четыре исходных файла совпали
с ревью. Общий CLI gate PASS 69.418 s; native проверка текущей сборки
выполняется в отдельном fresh context. Обычный CLI и старый memory-host режим используют Legacy/v1.

## Использование нового режима

```sh
minzc source.nanz -b z80 -t cpm \
  --memory-host-arena-append-policy result \
  --memory-host-compare-policy preserve-speed \
  --memory-host-workload workload.json
```

Policy обязательна: `legacy` или `preserve-speed`. Для выбора минимального
измеренного размера добавляется `--Osize`; это GoalSize selector при той же
policy, не смена backend options. Whole association v2 можно передать через
`--memory-host-association result/association-v2.json` в другой каталог output.
Профиль должен относиться к тому же executable/source/workload и пройти fresh
replay; сохранённая стоимость сама по себе не даёт разрешения преобразовать код.

Режим принимает один неизменяемый original `.nanz` source unit, Z80/CPM и code
output. Imports и несовместимые режимы отвергаются. Legacy/new флаги нельзя
смешивать; явно переданные пустые флаги не включают обычную компиляцию.
Source/association/workload и весь inventory выходов проверяются по canonical
paths и SameFile до измерения и непосредственно перед записью. Admission и
execution отказ не создаёт каталог результата; ошибки IO могут оставить
частичные файлы.

## Сохранённые результаты

`association-v2.json`, `selection-v2.json`, `decisions-v2.json`, source/workload,
selected ASM/BIN и четыре каталога кандидатов с baseline v2, ASM/BIN, final MIR
и allocation diagnostics. Decision связывает SHA **точных сохранённых bytes**
association и selection, native BIN и измеренные T/байты. `-o` сохраняет ту же
выбранную assembly. Новая генерация выбранного кандидата не выполняется.

Сохранённый executable:
`795f69cea3ed2cc2301a0a70aeabd29c70c3f0065273597e59cbdc58fc712c72`.
Source c7c42795; финальный пакет 9d520058. Полные Speed/Size используют
30 последовательных вызовов Self-Lanz; обе цели измерили все четыре варианта.
Все кандидатные payloads между целями совпали. Preserve-speed T по mask 0–3:
1132449 / 1132290 / 1076874 / 1076715; байты: 2001 / 2122 / 1979 / 2100.
Это выигрыш конкретного workload, не универсальный выбор.

Короткие policy1 collect/replay выбрали arena: 566 T / 2122 B. Association,
selection и внешний `-o` побайтно совпали. Явный Legacy v2 контроль: 583 T /
2121 B. Эти zero-body пробы проверяют transport/replay, не activation compare.
Malformed/source/import/policy/path refusals прошли, старые Legacy admission
тесты также прошли.

Аудиты 5d7b4bcf и b639a913 проверили exact state/results, 10 globals, три
ресурса, ordered host events, 11 исходных loop counters, typed source proofs,
options/context и native diagnostic SHA. Всего 330 native наблюдений: 20
candidate payloads и пять association baselines. Это не новый PC ledger и не
сертификат счётчиков преобразованных native циклов.

[Пакет](grace-memory-host-policy-cli-artifacts/README.md) ·
[Манифест SHA](grace-memory-host-policy-cli-artifacts/manifest.json) ·
[Независимый аудит](grace-memory-host-policy-cli-artifacts/independent-oracle/FINAL-AUDIT.md).
Все 358 payloads tracked; после переноса packaged saved-only audit повторно
PASS330. Полный parent selector pipeline **PASS 1241.045 s**. Текущая production
сборка `393af1d…` также дала те же actual full30 Speed/Size результаты; её
context проверен отдельно, старые профили не использовались. Публикация
подготавливается после завершённых gates. Далее — свежие измерения тех же четырёх workloads
корпуса с явной preserve-speed policy и отдельными контекстами старых сборок.
