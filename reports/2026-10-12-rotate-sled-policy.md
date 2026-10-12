# Rotate sled: Speed inline, Size только при выигрыше всего модуля

Базаbeb245eb. Exact original16_rotate_sled source/SHA и baseline ASM/BIN/MIR/native measurements заморожены до изменений. Раньше все режимы складывали3+RLCA в CALL, включая zero-byte-benefit run3, затем добавляли8B shared runtime. Speed получал+27T на каждый transformed run. Для original wholemodule такое решение одновременно хуже по времени и размеру.

В Z80CodegenOptions добавлен OptimizeForSize, проведён через оба ordinary pipeline emission entry points. Speed/default оставляет RLCA inline. Size учитывает весь модуль: только runs4..7, суммарная экономия bytes(count−3) должна строго превышать8B runtime. Equal-size выбирает inline; неподдержанный count8+ не создаёт undefined __rotate_N. Уже существующий emitted CALL к sled снимает дополнительный overhead, но комментарий не считается CALL. Source и прочие peepholes не менялись. Runtime stack guarded path по-прежнему исключает этот post-emission pass.

| Original workload | До (все3режима) | После (все3режима) |
|---|---:|---:|
| Полный moduleBIN |192B|186B|
| main до source terminalHALT |1142T|1088T|
| swap helperCALL |70T|43T|
| rol3 helperCALL |66T|39T|
| high helperCALL |77T|50T|

Low/rol1 неизменны. Все256входов пятиhelpers+mainstdout1861011 прошли native3modes, helperPC/SP, mainHALT/SP, fullRAM/profilerwrites, IX/IY. Size в этом original не выбирает sled, потому что savings2B не покрывает8B runtime. Дополнительный native nine-site testcase доказывает положительную Size-ветвь: fullBIN37→36B включаяruntime, все256x и registers/SP. Synthetic testcase — только правило выбора, не новое приложение.

Focused originalPASS0.028s; targeted existing ASMpeepholes+policytestsPASS0.011s. Baseline/selected artifacts сохранены отдельно, broad pipeline/fullMIR не повторяли. Нет universal оптимальности и нет новогоABI. Profile Options уже включает OptSize; новый CodegenOptions переносит существующую цель, не добавляет пользовательский searchmode.
