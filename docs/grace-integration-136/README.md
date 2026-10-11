# Интеграция136: signed widening для допустимых физических homes

Код9b20ab89, donor f4cc9911 после unsigned extension. Исправлен genSextValue для i8/i16/i24→16/24/32бит: source snapshot до destination writes, корректные sign bytes, shadow/index/spill и перекрытия. [Полный отчёт и original physical FAIL](../../reports/2026-10-11-signed-wide-extension.md). Default source до исправления проходил; это доказанный сбой допустимых emitter homes, а не заявленный default-source miscompile.

Root **100 368 native физических случаев PASS14,906 с**, 148homes, fullA/F/AF′, остальные registers, exactRAM/SP; исходниковые directcast/helper CALL проверки raw/finalVM/native **PASS0,086 с**. [Лог](root.txt). Все255 payloads donor manifest повторно провереныSHA256; пакеты целиком скопированы на шару. Isolated fullMIR/HIR PASS72,053/24,435 с относятся donor checkpoint. Root fullcore c69e350e предшествует signed fix; новый полныйPASS ему не приписывается. CLI c69e350e и pipeline a7954042 ещё выполняются отдельно.

Исследование originalwidemath продолжается: frozen abs16 контрпример frontend и newtype_area native mismatch требуют самостоятельных исправлений. Общая цель не закрыта.
