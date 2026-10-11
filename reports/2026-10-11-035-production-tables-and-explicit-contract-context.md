# Checked-local таблицы в обычной компиляции

Интегрированы `5652212c` и `a3d9452b`. Обычный CLI принимает
`--Osize --checked-local-table table.json`; CompileHIRSteps использует тот же
путь. Только pure single-block leaf с поддержанным return ABI получает
кандидат. Проверка связывает функцию, terminator, контракт, parameter allocation,
pattern descriptor и стоимость. Несовпадение оставляет исходный PBQP.

На source add5(x) все 256 входов native проходят с правильным результатом и SP.
Собранный модуль уменьшается с 8 до 6 байт, fixture — с 56 до 49 T. CLI BIN
совпадает с выбранным BIN. Это инфраструктурная проверка; польза таблиц на
реальной программе остаётся следующим заданием. CFG/calls/flags не включены
в область допуска. Таблица перечитывается на каждую компиляцию; mutable external
path явно отвергается профильным выбором, пока его содержимое не связано с proof.

Root повторил новые production/admission tests: VIR 0.016 s, pipeline 0.077 s,
exit 0. Полный VIR агента прошёл; общий pipeline checkpoint cb587fe4 ещё работает.

Разобрано известное расхождение TestMultiFunctionProfileVMZ80: при explicit
Options с ContractOpt=false после исправления API long_sum получает 1690 T
вместо исторических 1455. Независимый overlay с единственным изменением
ContractOpt=true восстанавливает 1455 и весь тест проходит. Fixture теперь явно
задаёт свой прежний contract-optimized контекст. Численный эталон не изменён;
false продолжает означать отключённую оптимизацию. Root совместный повтор
fixture и regression ContractOpt: exit 0, 0.197 s.

[Артефакты](../docs/grace-production-local-tables/README.md),
[root tables repeat](../docs/grace-production-tables-integrated.txt),
[explicit context repeat](../docs/grace-profile-explicit-contract-context.txt).
25 файлов сохранены и сверены SHA256 на шаре; integration-035-sha256.json.
Общая цель активна. Следом реальные callers для таблиц, расширение native
оракулов и безопасный общий вынос invariant address expressions.
