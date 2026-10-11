# Интеграция 157: исходный Lizp DSP на VM и Z80

Неизменённый `examples/lizp/dsp.lizp`: все пять функций проверены независимым host-оракулом против raw MIR VM, final MIR VM и собранного Z80. На каждый из трёх режимов — 282 368 настоящих CALL, всего 847 104. `lowpass`/`peak`: все 65 536 пары; `square_wave`/`sawtooth`: все пары с ненулевым period; `clamp_signal`: все 256 x и 81 сочетание границ, включая lo > hi с порядком cond из исходника.

Оракул: Book v7 §3.1 (ширины), §21.2 (Lizp→Lanz), явные операции исходника. Каждая u8 операция усекается до следующей; `lowpass(200,100)=22`, а не 150. Это byte DSP, не widened/аналоговый DSP. Period0 и полный clamp 256³ не проверены, поэтому каталог получает **bounded**, не exhaustive статус. Проверены возвраты, восстановление PC/SP, неизменность кода и всей 64 KiB памяти за исключением реально использованных 10 байт стека; spills нет. Все режимы — 116 байт BIN, выигрыша оптимизации здесь не заявляем.

[Источник, ASM/BIN, raw/final MIR, callable JSON, CALL drivers, summary и SHA256](../../reports/lizp-dsp-native/manifest.json). Root gate PASS; каталог сохраняет все 194 пути: 48 independent + 7 bounded = 55, pending 82. Это не доказательство всего корпуса.
