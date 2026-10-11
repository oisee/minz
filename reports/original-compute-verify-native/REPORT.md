# Исходная Z80 compute verification

Все 21 исходные функции z80_compute_verify.nanz проверены без изменения исходника: 1,840,989 реальных native CALL в ordinary, Grace Size и Grace Speed. Raw и final VM независимо сверены с host арифметикой по модулю 256/65536; возврат проверен в фактическом ABI. Active source assertions сохранены и отдельно выполняются CompileHIRSteps в исходном via-z80 режиме.

Полные byte pairs проверены для add/sub/AND/OR/min/max/abs; десять unary функций — все 256 входов. Clamp: 256 значений x и 9×9 заранее выбранных границ, включая lo>hi с исходным порядком условий. Word add/sub: все 65536 low-byte пары плюс 12×12 полноразрядных границ. Sum_to: 0–254; для 255 byte-счётчик бесконечно заворачивается, поэтому отдельно проверены gas/step-budget nonreturn VM и native. Это не все пары u16 и не полный clamp Cartesian.

Закомментированные ожидания count_bits и is_power_of_2 покрыты независимыми полными unary доменами. Закомментированный bxor отсутствует среди source callables и не включён выдуманным helper.

Каждый CALL проверяет return PC/SP и точные stack return bytes. Весь 64KiB вне наблюдаемого stack и явно объявленных compiler spills проверен на отсутствие native writes и изменений; у текущих сборок spills нет. Raw/final VM не выполняют data accesses. Retained executable PASS за 4,04 с; SHA256 f9b15f78c489051e73ec3d4fe25477b2b5b9388013759dec3afc39537085f844. Полные ASM/BIN/MIR/ABI/results и source freeze сохранены. Это correctness packet, а не новый heldout выигрыш Grace.
