# MIR2: безопасное слияние живых групп

Production `35ee3d6f` (donor167b66d3) добавляет проверку interference всех исходных участников coalesce groups. SSA edge argument может оставаться живым после внутреннего цикла; прежнее предположение о disjoint live ranges неверно. Existing backedge policy сохранена.

Исходный Tetris clear_lines на четырёх заполненных нижних строках возвращал raw4/final1. Exact source/input/raw/final MIR/trace до исправления сохранены в [пакете](../../reports/tetris-precoalesce/REPORT.md). После исправления агентский original CPM native/raw/final в трёх режимах проходит, полный TUI proof ещё выполняется.

Root targeted Coalesce/Contract PASS 0,029 с; nested counter и transitive-group regression включены. Это focused проверка production fix, не весь MIR2/pipeline и не benchmark улучшения. Полный gate и независимые native heldout после изменения требуют отдельного результата.
