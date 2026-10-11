# LIR удалён из живого компилятора

База `c3864cf2`. Каталог `minzc/pkg/lir` (47 Go-файлов), последний pipeline
research test consumer, прямые HIR research ветви и исторические z80spec
FromLIR/ToLIR views удалены. VIR, canonical z80spec и shared rewrite engines
сохранены. Production машинный код/allocator этим шагом не переписывались.

## Сохранение research материалов

`research-lir-c3864cf2.tar.xz` содержит точные 47 файлов LIR и шесть внешних
consumers из базы. `archive-manifest.json` связывает каждый из 53 файлов с SHA256
и коммитом. Архив повторно открыт, все 53 содержимых хеша проверены. Он не входит
в Go build graph. Старые wrong-code observations, LIR VM/WFC/Z3/RISC research
fixtures воспроизводимы в архивном checkout; не выдаются за текущие native PASS.

## Активное покрытие и перенос

- GCD default/Grace, arithmetic/comparisons, casts, shifts и word/dword cases,
  negative controls, ABI/body mutation и deterministic mismatch ordering
  остались в `hir/exhaustive_judge_test.go` и `native_lowering_judge_test.go`.
- Шесть critic composition sources и независимые модели сохранены в
  `hir/native_composition_test.go`: два byte arguments исчерпываются, остальные
  принимают 3/4. Полные research Cartesian sweeps (до 2^32) архивированы;
  это не заявление полного Cartesian native покрытия.
- Все 32 прежних Combine ISLE declarations сохранены как shared matcher
  `rewrite/isle/testdata/legacy-combine.isle` fixture. Тест разбора/identity
  показывает работу общего движка, но не легальность memory-fusion правил.
- Сохраняются shared `rewrite/sexpr`, `rewrite/datalog`, `rewrite/isle`,
  z80validate и asmutil; canonical overlaps/halves/kinds и VIR/MIR views
  продолжают тестироваться.
- Known-gap test в z80spec больше не объявляет исправленные IX/IY containment
  ошибки известными: удалены только восемь устаревших gap records; общая
  проверка actual relation против canonical spec осталась строгой.

Это завершение **удаления пакета**, а не всей цели переноса оптимизационной
пользы. Инвентарь `docs/grace-lir-removal-inventory` остаётся основанием следующих
работ. Byte multiply coefficients уже дублируются native backend. Word10/12 после этого snapshot перенесены с измеренной native пользой;
[результат](../grace-mul16-10-12/REPORT.md). Effect-safe load/store16 fusion теперь перенесена для проверяемой same-block
формы: [FAT12 и ограничения](../grace-fat-word-fusion.md). Полнота
constraint/pattern переноса ещё не доказана.
[Актуальный статус инвентаря](../grace-lir-removal-inventory/README.md).

## Проверки

Go 1.26.8, `GOCACHE=/tmp/minz-grace-gocache`:

- полный `go test ./pkg/hir -count=1`: **PASS, 34.150 s**;
- полный `go test ./pkg/rewrite/isle ./pkg/z80spec -count=1 -v`: **PASS**;
- `go test ./pkg/... ./cmd/... ./internal/... ./tests/... -run '^$'`: **PASS**,
  это compile-only всех поддерживаемых toolchain packages, без runtime tests;
- первоначальный `go test ./... -run '^$'`: **FAIL**, только старые standalone
  `scripts` mixed-package и `scripts/analysis` relative-import errors. Исходный
  лог сохранён; не маскируется как общий PASS и не исправлен в LIR cleanup.

`deletion-audit.json` проверяет отсутствие живых LIR imports и каталога.
Исторические source snapshots в reports/docs не изменяются.
