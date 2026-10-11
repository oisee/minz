# Production pipeline больше не зависит от LIR

База `33146966`. Удалены `Options.LIRCheck`, `Options.UseLIR`, `Steps.LIRResults`,
production вызов convergence, disabled-backend stderr/trace shim и CLI no-op
`--lir`/`--z3`. Старые флаги теперь отвергаются как неизвестные. Production
pipeline и сохранённый VIR не импортируют LIR; Go dependency list pipeline
сохранён рядом и не содержит `pkg/lir`.

Независимые native проверки сохранены: GCD default/Grace, байтовые арифметика и
сравнения, shifts, word/dword arithmetic, casts, multiplication, compositions,
negative controls и порядок диагностики. Только повторный запуск идентичного
PBQP-варианта под удалённым `--lir` исключён. Названия native тестов приведены к
реальному backend. Опциональные research LIR тесты пока остаются и явно вызывают
research API сами; не могут включить LIR через production Options.

## Конфигурация профиля

`ProfileCompilationID` сериализует весь Options. Удаление двух false полей
изменяет идентификатор даже без изменения машинного кода:

- старый DefaultOptions: `4ada88316c49dcab114118447a4c5d1e5d18e1ffed84a4a9c80375e7ebc7750b`;
- новый DefaultOptions: `0478cefaf67edd54bf36efc90fafaa27da51661c2f0c3294e27f6c941d02860c`.

Новый regression реконструирует прежнюю сериализацию и проверяет **отказ** от
старого measured model до применения преобразований. Исторические evidence IDs
не менялись; новые profiles/decisions нужно собирать в новом контексте. Это не
попытка объявить старый профиль совместимым через смену ярлыка.

## Проверки

Go 1.26.8, `GOCACHE=/tmp/minz-grace-gocache`:

- `focused.txt`: новый configuration-refusal и retired-flag tests, shared extern
  stubs, прежние research fixtures, unsigned pair policy admission/source — PASS.
- `profile-cli.txt`: multi-function profile VM+Z80 и его 11 отказных вариантов;
  memory-host/policy/search CLI admission — PASS.
- `native.txt`: native single-block full byte pair domains; shift counts1/3;
  word/dword arithmetic; 17 lowering cases (включая 524288-input logical case);
  six compositions; negative controls и ordering — PASS.
- `gcd.txt`: все 65025 положительных byte pairs для default и Grace — PASS.

Полный медленный pipeline и весь корпус этим фокусным прогоном не объявляются
пройденными. Удаление самого `pkg/lir` остаётся следующим этапом после завершения
переноса/архива research test consumers и полезных возможностей.
