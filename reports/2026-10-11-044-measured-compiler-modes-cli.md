# CLI выбирает измеренный режим компиляции

Интегрирован `eaf8c8b7`. Новый переключатель `--hir-profile-compiler-modes`
использует существующие `--hir-profile-auto DIR` и
`--hir-profile-auto-workload workload.json`. Нужен `-t cpm`; выход `-o`
поддерживает .a80 или .bin и содержит точный измеренный победивший артефакт.

Пример из корня repo после сборки mz:

```sh
mz examples/nanz/pfcco_weighted_packet.nanz -t cpm   --hir-profile-auto /absolute/output   --hir-profile-auto-workload docs/grace-pfcco-prospective-heldout/artifacts/training-workload.json   --hir-profile-compiler-modes -o /absolute/selected.bin
```

Измеряются четыре сочетания ContractOpt/MIR Grace. Speed — минимум тактов без
штрафа за байты. --Osize выбирает цель Size; compiler OptSize у всех кандидатов
в этом режиме остаётся false, чтобы не смешивать цель с политикой проходов.
Это controlled calls BIN, не linked CP/M COM.

Root repeat четырёх сочетаний goal/output, import oracle и alias refusals:
PASS 4.260 s. Speed выбирает mode1: 2605→2320 T при52 B.
Size выбирает original по равенству52 B. Portable Python проверяет все
кандидаты, arithmetic/state/byte accesses, SHA и точное совпадение output.

Архивируются source, imports, workload, selection, каждый ASM/BIN/MIR,
selected outputs и manifest. Неоднозначные или отсутствующие import paths
отвергаются; imports разрешаются по фактическим frontend hashes.
Запись файлов пока не atomic; staged publication остаётся следующей работой.
Это CLI повтор прежней training нагрузки, не новый heldout эксперимент.

[Отчёт](../docs/grace-compiler-modes-cli/REPORT.md),
[root repeat](../docs/grace-compiler-modes-cli-integrated.txt).
Все50 manifested файлов и frozen CLI сохранены на шаре с SHA256.
Полный pipeline cb587fe4 ещё выполняется; общая цель активна.
