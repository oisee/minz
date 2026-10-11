# Интеграция 160: исходный PL/M ABS_DIFF/FIB

Неизменённый `examples/plm/abs_diff_fib.plm`: обе функции с полными byte-доменами. ABS_DIFF — 65 536 пар, FIB — все 256 n. Независимый host-оракул абсолютной разницы и математической Fibonacci recurrence modulo256 сравнивается с raw MIR VM, final MIR VM и собранным Z80. Book v7 §21.5 явно задаёт BYTE→u8; ширины — §3.1.

На режим 65792 настоящих CALL, всего 197376; ordinary/Grace Size/Speed PASS 0,698 с. BIN ordinary 47 байт, Grace Size/Speed 38 байт (−9 байт), full RAM и код неизменны кроме реально использованных 6 байт стека, spills нет, PC/SP восстановлены. Это проверка полного исходного callable-домена, не всей среды PL/M/CP/M ; уменьшение размера подтверждено, такты не измерены.

[Исходник, ASM/BIN, raw/final MIR, callable metadata, CALL drivers, summaries и SHA256](../../reports/plm-absdiff-fib-native/manifest.json). Каталог: 194 пути, 50 independent +8 bounded =58, pending79. Общий struct-return ABI в работе отдельно.
