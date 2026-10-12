# Полные frontend/HIR/VIR проверки

Checkpoint454f709b: полные HIR, Lanz, Lizp, PL/M, Pascal, C89, Frill и VIR **PASS**. HIR 31,288 с; остальные времена приведены в [неизменённом логе](frontends.txt). Это пакетные проверки после production HIR pointer/print/addr_of/BIT исправлений. MIR2 отдельно PASS84,903 с; полный pipeline94a41120 ещё работает.

TUI ZX проходит исходный main на native с явной моделью двух ROM-входов: 194 ожидаемых output bytes, full RAM, настоящий DI/HALT. Все три режима603B/9314T. Это native ROM protocol, не экранные пиксели/реальное железо и не VM differential. [Подробности](../../reports/tui-zx-main/README.md).
