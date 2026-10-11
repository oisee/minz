# Native Z80: полный динамический счётчик сдвига

Следом за `d5abef7a` исправлен отдельный реальный miscompile: `global a:u32; global b:u32; result=a<<b` при a=1,b=256 возвращал 1 на Z80 вместо 0 в raw/final MIR VM. Контрпример и прежний ASM заморожены в предыдущем пакете `artifacts/u32-native/dynamic-shift-negative.*`.

Причина — `genVariableShift` брал только младший байт счётчика (`LD B,C`). Теперь он проверяет все байты исходного счётчика, включая теневой банк и spill-слоты, прежде чем перезаписать scratch counter. Если величина достигает ширины результата, цикл выполняет ровно ширину сдвигов. Так логические сдвиги очищают результат, арифметический правый сохраняет знак, а большие счётчики не создают длительные циклы. Ноль остаётся тождественным сдвигом. A восстанавливается после проверки, scratch pair — после цикла.

Language Book v7 §§3.1–3.4 определяет ширины и представление чисел; точной нормы для счётчика >= width там нет. Исправление сохраняет существующую семантику raw/final MIR VM (`OpShl`, `OpShr`, `OpSar`) и независимого Go width oracle; оно не вводит modulo-count поведение. Не меняются greedy/PBQP и cache schemas.

## Доказательство в ограниченной области

- 19 ordinary Nanz sources, по 84 × 84 = 7 056 пар каждый: 134 064 пары через raw VM, final VM и native Z80. Новые dynamic SHL/SHR включают 31/32/33/63/64/255/256/257/65536, старшие биты и инвертированные walking bits. Остальные 17 источников сохраняют ADD/SUB/logic/constant shifts/derived rotates/call-live-u32 проверки.
- 2 184 forced native случая: u16/u32 SLA/SRL/SRA, счётчик в BC, IX, A или памяти; 7 значений операнда и до 15 счётчиков. Проверены независимый width/sign oracle, сохранность source, A, BC/IX счётчика и SP.
- Focused pipeline / MIR2: PASS, 1.899 / 3.217 s. Полный MIR2: PASS, 17.338 s (`u32-shift-full-mir2.txt`).

Точные источники/ASM/BIN и SHA256 находятся в `artifacts/u32-shift/manifest.json`. Старый logic packet не переписан. Новые программы — контрольные fixtures, не полноценная SHA256 workload; оптимальность кода и PASS всего pipeline не заявлены. Constant non-HL left-shift path остаётся отдельным read-only подозрением; этот фикс его не меняет.
