# Nanz: отрицательные литералы и original rotozoomer

Root повтор exact-source четырёх чистых функций rotozoomer и отрицательных литералов PASS 4,418 с; полный Nanz PASS 21,771 с. Теперь i16 return -11 даёт FFF5, а не 00F5. Литералы сохраняют signed numeric value в типизированных контекстах; unary minus переменной u8 сохраняет прежнюю byte-wrap семантику. Проверены границы -128/-129/-32768, return, locals, explicit casts, literal arguments и условные arms.

Original abs16 покрыт всеми word patterns; axis_a/axis_b — всеми byte phases; texture — замороженным bounded grid. Всего 200 364 native и 400 728 VM проверок в трёх режимах. Полный исходник сохранён; native rendering/main не заявлены. Старый GIF сохранён отдельно с SHA; новый получен независимой signed16 моделью и проверен на всех 32 кадрах плюс wrap в VM.

Отчёты, raw/final MIR, ASM/BIN и root логи сохранены здесь и на шаре. Это исправление correctness, не измеренный новый Grace speed gain.
