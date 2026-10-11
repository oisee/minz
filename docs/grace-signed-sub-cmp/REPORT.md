# Исправление signed Sub/Cmp fusion

База `33146966`. Исходник `reports/showcase-src/2026-03-10/ex4a_abs_diff_iar.nanz` не изменён.

Raw HIR/MIR2 корректно вычислял `abs_diff(20,-10)=30`. После MIR оптимизации `CmpGe(a,b)` превращался в `CmpSubCarryNot(a-b,b)`: это unsigned borrow, а не signed S xor V. Результат стал 226 на VM и в native инструкции SUB/C-branch. Для -128/127 получалось 1 вместо 255.

Исправлены два преобразования `condret.go`: непосредственная Sub/Cmp fusion и reorder последующего Sub перед Cmp. Signed ordering сохраняет исходный compare; существующий Z80 codegen нормализует S xor V. Equality и прочие условия больше не получают случайно изменённый lhs без изменения предиката. Unsigned и legacy unsigned CmpLt/Ge fast paths остаются.

Спецификация ожиданий: Nanz Language Book v7 §3.2 — signed i8 comparison; §3.1 — stored byte widths. Расстояние между signed byte значениями 0..255 возвращается как u8; byte subtraction/return проверяются modulo 256.

## Проверка

Матрица a,b ∈ {-128,-127,-10,-1,0,1,10,126,127}, оба порядка и equality; пять функций: исходный abs_diff, signed less-with-sub, signed equal-with-sub, signed greater-with-sub, unsigned control.

405 случаев для обычного MIR и 405 для Grace MIR: **810 PASS raw MIR2 + optimized MIR2 + assembled Z80**. Это 2430 успешных функциональных проверок на трёх стадиях (включая 810 native вызовов). Oracle вычислен отдельной Go signed/byte арифметикой; исходные ассерты не ослаблены.

Unit contract проверяет signed Lt/Ge, Eq/Ne/Gt/Le, unsigned Lt/Ge и legacy unsigned Lt/Ge; отдельная reorder регрессия покрывает signed отказ и сохранённый unsigned путь.

Полный `pkg/mir2`: PASS. Исходный showcase abs_diff: PASS, 31 bytes BIN, прежний stale skip удалён в этой базе. Parent с точным path skip audit должен удалить соответствующий exact-path entry при интеграции.

Сохранены source, raw/final MIR, ASM, BIN, 810 oracle CSV rows, before/after diagnostic и логи. ASM/BIN — whole candidate module, не отдельные bootstrap образцы каждой из 810 native assertions; native assertion runner выполнял реальные assembled products со своим ABI bootstrap. Полная матрица всех 65536 входных пар и hardware в этой работе не запускались.
