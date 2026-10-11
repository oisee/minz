# ENRT checkpoint index: поздние формы без длинного prefix scan

Добавлен отдельный offline индекс контрольных точек для настоящих полных ENRT4v/5v. Чтение поздней формы `hash16` теперь занимает один блок **59 767 B**, а не упирается в prefix limit 256 MiB. Это устраняет ограничение поиска, но **не включает word XOR**: assignment по-прежнему отклоняется текущим независимым допуском; checked таблица для hash16 пуста.

Для `triple` адресный блок 27 461 B (раньше prefix 4 107 216 B), а результат unchanged: полный original09 BIN34→31B, прямой66→54T и косвенный123→111T на всех256 входах VM/native, SP/code immutable. Таблица используется ordinary CLI.

## Что связано и что доверяется

`enrt-checkpoints/v1`: canonical ENRT header/count/layout, full source SHA256/size, stride4096, block first/count/offset/length/SHA256. Offline builder проходит все records один раз, проверяет vreg count по canonical enumeration и EOF; источник ограничен600MiB, index2MiB, block≤139264B. Число blocks:4v39,5v4240. Для4v полныйSHA `7b49113c8a885e4fbe6bf0154ff11fbf88426f0d3daacc7ac019408dd4617a58`; полный5vSHA сохранён в index5.json. Dense6v/OFB не поддерживаются.

Operation открывает index+ENRT, копирует ограниченный index, проверяет монотонные contiguous offsets/count/length и hash **всего открытого исходного файла один раз**. Каждый Import читает один block и проверяет его hash и полный record parse до AddChecked. Перед публикацией operation.Validate повторно hash всего **пути источника**, поэтому замена файла при старом retained descriptor тоже обнаруживается. Source metadata не служит доверенным cache. Итог2 full-source hashes на converter operation, не на каждую функцию.

Сам sidecar считается **недоверенным предложением адреса**. Его full-source SHA не доказывает, что злонамеренно переписанные contiguous block boundaries соответствуют каноническим offsets оригинального builder. Даже тогда assignment не получает сертификата по index metadata: полный текущий `AddChecked` сохраняется. Evidence `vir-checkpoint-enrt-proposal/v1` показывает предоставленный record index/block hashes и source SHA; это не доказательство исторической оптимальности или аутентичности sidecar. Для canonical provenance используется exact sidecar, созданный полным builder и сохранённый в packet с SHA manifest.

Таблицы API Import provisional до успешного финального Validate. Converter публикует только после него. Выводы записываются полными temp bytes через hardlink no-overwrite; две публикации последовательны, при ошибке второй первая удаляется. Это не транзакционная гарантия при process crash. Финальный hash проверяет source на момент validation; произвольная конкурентная запись после завершения не меняет уже скопированные certificates. Исходные текущие функции/ops/ABI не ослаблены.

## Измерение одного прогретого host запуска

| Действие | Wall time | Max RSS процесса |
|---|---:|---:|
| Полный offline build4v (~4MiB) | 0.04s | 14592KiB |
| Полный offline build5v (~387MiB) | 5.81s | 17676KiB |
| Final converter hash16 + source compilation/full hashes | 1.77s | 19524KiB |
| Final converter original09 + source compilation/full hashes | 0.45s | 18432KiB |

Первоначальные converter samples 2.02s/0.69s включали лишний предварительный final hash, затем удалённый; оба набора сохранены, таблица показывает final вариант. Индексы≈8KiB/796KiB. Ранее `hash16` prefix scan в corpus run занял≈15.3s и не дошёл до записи. Это разные однократные host samples, не строгий benchmark cold I/O. Нет обещания, что index быстрее раннего lookup: initial/final full SHA может доминировать, особенно для одной ранней функции. Преимущество — bounded block lookup и amortization на несколько функций операции.

## Команды

Из `minzc/`:

```sh
go run ./cmd/import-enrt --enrt /mnt/safe/z80-compiler/tables/enriched_5v.enr \
  --build-index /path/new-index5.json

go run ./cmd/import-enrt --source ../examples/nanz/09_function_pointers.nanz \
  --enrt /mnt/safe/z80-compiler/tables/enriched_5v.enr --index /path/new-index5.json \
  --out /path/new-table.json --evidence /path/new-evidence.json
```

`--quotient` совместим с `--index`. Build mode принимает только --enrt и новый --build-index output; generation output не перезаписывает существующие файлы.

## Проверки

Canonical synthetic full4v stream; malformed offsets/order/count/schema/sourceSHA/blockSHA; selected-block in-place mutation; same-size pathname replacement вне выбранного block; truncated/invalid/trailing source; source fullSHA and block hashes; real historical triple nativeall256 direct+indirect. Full VIR отдельно. `reports/artifacts/enrt-checkpoint-evidence.tar.xz` содержит exact index4/index5, SHA manifest, источники original09, ASM/BIN/currentcert, timings/full5videntity и отрицательный hash16 packet. Полный pipeline PASS не заявляется.
