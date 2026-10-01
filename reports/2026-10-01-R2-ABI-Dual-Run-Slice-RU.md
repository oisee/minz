# R2.2: первый ABI-aware dual-run с production PBQP

Срез 2026-10-01. Это **первый срез R2.2**, а не закрытие всего ABI/call/flag/clobber трека.

Тестовый runner теперь берёт порядок и типы параметров из `MIR2 Func.Contract.Params`, а их реальные физические места — из `AllocResult`. Возврат читается по `Contract.Returns`. Неподдерживаемая форма ABI даёт явную ошибку, а не ложный PASS. `pipeline.Steps` теперь предоставляет именно тот `Allocation`, с которым production PBQP вызвал Z80 codegen; повторной аллокации в тесте нет.

Ручные bootstraps из синтетического HIR oracle и signed comparison corpus заменены этим runner. Тем же путём исполняются `nested_u8`, `signed_less`, `wide_add`, `abs_diff`; последним двум добавлено Z80-покрытие. Отдельный production corpus проходит HIR → реальные optimization passes/contract optimization/PBQP → ASM → MZA → эмулятор и сравнивает наблюдения с оптимизированной MIR2 VM и независимой арифметикой Go. Отрицательный контроль с переставленными ABI-регистрами обнаруживается oracle.

Production corpus выявил новый wrong code: прямой `return a < b` с возвращаемым типом `bool` давал на Z80 байт `255` при истинном результате, тогда как VM даёт `1`. Причина — общий `F → A` путь через `SBC A,A`; исправленный Z80 return path материализует фактический предикат в канонический `0/1`. Проверены `< <= > >=` на знаковых границах.

Граница покрытия: runner сейчас загружает параметры шириной 8/16 бит в обычные или IX/IY регистры; spill/stack/shadow параметры отвергаются. Возврат через `ClassFlag` проверен на отдельном hand-built MIR2 signed-compare с PBQP/ASM/эмулятором; другие flag predicates, сохранность clobbers, несколько результатов и 24/32-битные ABI ещё впереди. Синтетический `compileHIRFixture` сохраняет прежний greedy путь; отдельный production corpus использует PBQP. Реальные source frontends и CP/M executable в этом срезе не проверялись.

Локально пройдены stable Go CI subset и Nanz suite с прежним исключённым showcase test; точные команды приведены в [хэндовере](../docs/project/HANDOVER-2026-10-01.md#начать-здесь-после-перезапуска).
