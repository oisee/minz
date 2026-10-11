# Сохранённый профиль: принятие и точные отказы CLI

На неизменённом Self-Lanz и сохранённой production CLI проверен отдельный
вход `--memory-host-profile`. Один настоящий профиль текущей сборки принят,
четыре остальных отклонены с ожидаемой причиной и без каталога результата.
Production source, compiler и разрешения memory-host не изменены.

| Вход | Происхождение | Фактический результат |
| --- | --- | --- |
| Current profile | Настоящий здоровый профиль 1bd955 | Accepted, arena-only, 763 T / 2212 B |
| Stale executable | Настоящий здоровый профиль 852cf | incompatible family/source/workload |
| Failed sample | Явно синтетическая completion/failure mutation | incomplete or failed sample |
| Failed invocation | Явно синтетическая invocation mutation | failed invocation |
| Cumulative counts | Явно синтетическая count mutation | cumulative count mismatch |

Два настоящих профиля отличаются только двумя полями compiler identity;
исходник, имя source unit, workload, наблюдения и счётчики совпадают. Это
изолирует несовместимость сборок. Синтетические входы сохраняют текущий
context и меняют только объявленные поля. Они проверяют порядок отказов
загрузчика, а не доказывают происхождение настоящего failed execution.
CLI не экспортирует failed profile; такой exporter здесь не добавлялся.

У каждого отказа exit code 1; requested output directory, selected BIN и
accepted decision отсутствуют. Корректный supplied profile повторён перед
выбором, четыре actual кандидата проходят независимую модель исходника:
10 globals, три RAM-региона, ordered byte events и 11 исходных loop counters.
Typed precompile proofs, function/source maps, native sidecars/call frames,
allocation/MIR/ASM/BIN SHA и точный SHA JSON решения проверены отдельно.
763 T относится ко всему короткому workload, не к одному source loop.

Это artifact-only этап: предыдущие pipeline 569.037 s PASS и CLI tests
30.102 s PASS сохраняют прежнюю область проверки; заново их не запускали.
Новых assertions о standalone extern ABI или hardware нет.

Артефакты и команды: [evidence README](grace-memory-host-supplied-artifacts/boundaries/evidence/README.md),
[точные входы](grace-memory-host-supplied-artifacts/boundaries/input-inventory.json),
[независимый аудит](grace-memory-host-supplied-artifacts/independent-oracle/FINAL-AUDIT.md),
[native review](grace-memory-host-supplied-artifacts/native-review/FINAL_AUDIT.md),
[манифест SHA256](grace-memory-host-supplied-artifacts/manifest.json).
26 крупных payloads сохранены deterministic gzip с SHA исходных bytes;
бинарники и обе разные executable сборки сохранены локально.

Проверка после переноса, без повторного CPU:

```sh
python3 docs/grace-memory-host-supplied-artifacts/boundaries/evidence/independent-oracle-review/audit_supplied.py docs/grace-memory-host-supplied-artifacts/boundaries/evidence /tmp/supplied-profile-audit.json
```

Следующий шаг: actual per-PC стоимость функций Self-Lanz при неизменённых
проверенных бинарниках. Inclusive/exclusive attribution и adapter/caller
стоимость должны быть явно разделены; source-loop lineage не предполагается.
