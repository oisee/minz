# Интеграция 169: nested copies, owned metadata и исходный Widemath

Rootd0fc2059/d063c2a7 интегрируют6374d351/622de7f6. Nonliteral nested StructTy поля теперь копируются inline по полному layout вместо address-sized store; клонирование call ABI metadata владеет type trees и slices, primitive identity сохранена. Корректный Widemath observer декодирует byte обращения через BC/DE/HL/index, сохраняет ordered public W8 input/output effects, проверяет distinct private construction, explicit guarded6200 result buffer и full RAM/code/SP.

Совместный rootpipeline **PASS2,150с**, MIR2 focused0,024с; отдельный metadata mutation clone testPASS. Тест Widemath включает 35 841 raw/finalVM/native calls (11 947 ×3 режима), а struct regressions расширены nested_nonliteral. Это bounded original Widemath workload; прежний исчерпывающий snapshot не переносится на новыйABI без повторного исполнения. Ошибка старого observer сохранена отдельно.

Canonical reportpacket31payloads+manifest свереныSHA source/share. General profile aggregate value API остаётся explicit unsupported; full pipeline наd063c2a7 начинается отдельно. Общая цель ещё активна.

Followupcfaa7777 сохраняет точные recorded signature descriptors при clone, не исправляет подменённый ParamIndex. Negative clone control проходит; полныйpipeline запущен уже наcfaa7777 с сохранённымexactexecutable, не наd063c2a7.
