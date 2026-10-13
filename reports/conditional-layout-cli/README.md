# CLI: измеренный выбор сборки и проверка на новом входе

`mz --hir-profile-layout-modes FUNCTION` собирает восемь сочетаний
межфункциональной стратегии greedy/PBQP, HIR Grace и MIR Grace. Для одной
чистой условной функции сравниваются до четырёх допустимых порядков блоков.
Каждый кандидат — полный модуль Z80: такты учитывают реальные вызовы и
сохранение регистров caller. Победитель выбирается по обучающей нагрузке.

Без `--Osize` цель Speed минимизирует такты без ограничения размера, а все
режимы собираются с OptSize=false. С `--Osize` цель Size минимизирует полный
BIN и включает OptSize=true. При равенстве сохраняется более ранний вариант.

## Запуск

Из корня checkout с новым CLI:

```sh
go -C minzc build -o /tmp/mz-layout ./cmd/minzc
/tmp/mz-layout examples/nanz/grace_tokenizer_checked.nanz --target cpm \
  --hir-profile-layout-modes is_space \
  --hir-profile-auto /tmp/tokenizer-training \
  --hir-profile-auto-workload reports/conditional-layout-mode-selection/train.json \
  -o /tmp/tokenizer-selected.a80

/tmp/mz-layout examples/nanz/grace_tokenizer_checked.nanz --target cpm \
  --hir-profile-layout-modes-replay /tmp/tokenizer-training \
  --hir-profile-auto /tmp/tokenizer-heldout \
  --hir-profile-auto-workload reports/conditional-layout-mode-selection/heldout.json \
  -o /tmp/tokenizer-selected.bin
```

Для Size добавьте `--Osize` только к collect. Replay использует сохранённую
цель. Папки результата должны быть новыми; входы, импорты и выходы не могут
перезаписывать друг друга. `--hir-profile-layout-limit N` задаёт предел 1..4
раскладок при collect; replay использует сохранённый предел.

Collect сохраняет исходник, все импортированные source units, workload,
полное свидетельство кандидатов, точные `selected.a80`/`selected.bin` и SHA256.
`-o` получает выбранные байты, а не результат повторной обычной компиляции.
Replay сначала заново воспроизводит весь обучающий эксперимент и сверяет
MIR, ABI, назначения, варианты кода, замеры и выбор. Затем исполняет тот же
ASM/BIN на новом входе без выбора по heldout. Это строгая воспроизводимость,
но пока не ускоренный replay. Нужны тот же executable компилятора и тот же
checkout/логические пути исходников. Подмена замеров или ASM с пересчитанным
манифестом отклоняется; при отказе прежний `-o` остаётся целым.

## Настоящий standalone CLI на tokenizer

Неизменённый `grace_tokenizer_checked.nanz`: три обучающих и три отложенных
входа из предыдущей [API-матрицы](../conditional-layout-mode-selection/README.md).
Collect и replay запущены отдельными процессами одного собранного `mz`.

| Цель | Байты ordinary → selected | Обучение, T | Heldout, T | Победитель |
|---|---:|---:|---:|---|
| Speed | 4201 → 4138 | 334893 → 329689 | 289757 → 283909 | greedy + HIR Grace, ложный путь первым |
| Size | 4186 → 4120 | 334871 → 329844 | 289789 → 284072 | greedy + HIR Grace, обычная раскладка |

Ordinary — первый режим без HIR/MIR Grace и без подстановки раскладки.
Его ASM независимо пересобран и побайтно сверен с первым сохранённым
кандидатом; heldout исполнен отдельно. Speed даёт −5848 T и −63 байта,
Size — −5717 T и −66 байт. Эти числа относятся к этой программе и нагрузкам.
В прежней API-матрице даже цель Speed использовала OptSize=true; её baseline
и цифры нельзя подставлять вместо нового Speed CLI.

Все 32 кандидата Speed и 24 Size сверены с исходным HIR VM. Дополнительный
независимый Go-tokenizer проверил сохранённые результаты всех кандидатов,
оба selected heldout и оба ordinary heldout: токены, полную арену/буфер,
globals и точные количества доступов к каждому байту. Native runner проверяет
PC/SP и допустимые обращения. Побайтно проверены и все четыре `-o` продукта.

Направленные CLI, старый collect/replay и strict JSON: PASS за 33,825 с.
Независимый аудит настоящих CLI-артефактов: PASS за 1,643 с. Тесты включают
импорты, alias выходного файла, согласованную подмену тактов/ASM и duplicate
JSON keys. Ошибки не публикуют частичный результат и не заменяют прежний `-o`.

## Границы

Это опциональный режим Nanz → Z80 для CP/M. Он пока не принимает pinned
shape-таблицы через CLI, хотя underlying API умеет заданные конфигурации с
ними. Условная функция должна укладываться в проверенный чистый u8-контракт;
произвольные CFG, совместная раскладка нескольких функций и внешние runtime
режимы не поддерживаются. Обычные defaults не меняются, внешний solver не нужен.

Артефакты, компилятор и логи сохранены полностью на шаре:
`/mnt/safe/minz/grace/2026-10-13/conditional-layout-cli/manifest.json`.
В этом отчёте сохранены сжатые полные свидетельства и проверяющий Go-код.
Для повторного независимого аудита скопируйте `verify-artifacts_test.go` в
`minzc/pkg/pipeline`, задайте `MINZ_LAYOUT_CLI_ARTIFACTS` на каталог артефактов
и выполните `go -C minzc test ./pkg/pipeline -run '^TestLayoutCLIArtifactsIndependentTokenizer$' -count=1 -v`.
