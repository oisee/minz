# Единый независимый аудит Agon

Один инструмент теперь повторно разбирает все пять программ: graphics — 9 потоков и 9 debugger logs, console — 6 потоков и 6 debugger logs. Все 30 сохранённых запусков прошли. Нового исполнения CPU нет; это аудит исходных доказательств, не новый замер производительности.

Проверяются полное покрытие manifests и SHA256, точные потоки приложения, совпадение source/BIN между двумя видами запусков, сохранённые caller frames и фактический RET. Добавлена сверка идентичности BIN в обоих matrix summaries с файлами, а также source SHA в stream summary. Дубликаты случаев отвергаются даже при наличии всех нужных меток; четыре admission-теста PASS. Каждый режим аудитора также отвергает шесть повреждений исходных потоков/регистров.

В корне репозитория:

```sh
python3 scripts/agon_evidence_audit.py --wire reports/agon-native-vdu-matrix-d071b99c --returns reports/agon-debug-return-matrix-d071b99c --output /tmp/graphics-audit.json
python3 scripts/agon_evidence_audit.py --programs console --wire reports/agon-console-full-vdp --returns reports/agon-console-debug-return --output /tmp/console-audit.json
python3 -m unittest discover -s scripts -p test_agon_evidence_audit.py -v
```

Исторические пакеты не изменены. Ограничения сохраняются: bounded command/text и caller frame, без полного RAM/pixel-оракула или аппаратного запуска. Полный pipeline работает отдельным неизменным executable.
