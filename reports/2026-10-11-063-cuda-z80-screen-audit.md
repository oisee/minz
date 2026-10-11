# CUDA Z80: найденные инструменты и ограниченная проверка

На 11 октября GPU доступен: NVIDIA RTX 2070, 8 GiB, driver 550.120, CUDA runtime compatibility 12.4; установлен nvcc 12.0.140. Проверка использовала один блок из 256 потоков, 1 536 байт выходного буфера. Большой поиск не запускался; production compiler и CPU oracle не менялись.

## Что найдено

| Место | Что реально лежит |
|---|---|
| `/home/alice/z80_divmod.cu`, `z80_mulopt_fast.cu`, `z80_mulopt16.cu`, `gen_z80.cu` | Исторические CUDA исполнители коротких **enum instruction** последовательностей и перебор арифметических идиом |
| `/home/alice/z80_common.h` | Host-only executor 394 register operations, таблицы всех восьми флагов, fingerprints на 8/24 фиксированных состояниях |
| `/home/alice/z80search_v2`, `z80_focused`, другие `z80_*` | Исполняемые исторические binaries; инвентаризированы с SHA, неизвестные интерфейсы поиска не запускались |
| `/mnt/safe/@z80-optimizer` | Сжатые JSONL результатов и constraint-shapes распределения регистров, не исходник полноценного emulator |
| `/mnt/safe/@gpuforce` | Преимущественно AY atlas/audio/media; Z80 CUDA emulator source в обследованном каталоге не найден |
| `/mnt/safe/z80-compiler/tables` | Exhaustive register-allocation tables, enriched/OFB/IX варианты, арифметические библиотеки и peephole данные |
| `minzc/pkg/mir2gpu` | Компиляция MIR в CUDA/OpenCL/Metal kernels, не исполнение generated Z80 ASM |

`z80_divmod.cu` моделирует A, B, carry и alternate A/carry. Другие flags, полный alternate AF/EXX, PC, SP, stack, RAM, address wrap, ports/BDOS, interrupts, branches/calls не представлены. `z80_mulopt16.cu` дополнительно моделирует пары для ограниченных операций. Это полезный предварительный экран арифметики, но не полный Z80 CPU. `z80_common.h` шире по флагам, однако тоже без RAM/PC instruction decoding; его комментарий «verified bit-exact» — историческое утверждение, не результат текущего аудита всех операций.

Стоимость последовательности в CUDA поиске — сумма фиксированной `opCost`, не реальное исполнение branches/memory wait states. Для полноценного приложения этот способ не заменяет измеренные T-states CPU runner.

## Ограниченный настоящий GPU smoke

Обёртка `cuda-z80-screen/smoke.cu` включает **неизменённый** исторический `z80_divmod.cu`, переименовывает его search main и вызывает только recovered `exec_op`. Два случая, каждый на всех 256 значениях A, начальный B=0 и carry=1:

1. `LD B,A / ADD A,A / ADD A,B` — independent A_out = 3*A_in modulo 256, 12 T.
2. `SRL A / RRA / ADC A,B` — carry-sensitive последовательность, 16 T.

GPU A/B/carry совпали с CPU во всех 512 случаях. CPU reference отдельно собирает реальный ASM текущим `z80asm`, исполняет bytes в production Remogatto, проверяет все остальные primary registers, SP, PC, неизменность 64 KiB памяти и фактические 12/16 T. Полный итоговый F сохранён в CPU CSV; сравнение с GPU относится только к C, потому что остальные флаги там отсутствуют. Нет утверждения о поддержке всех opcode, live registers, вызовов или correctness произвольного кандидата.

Один холодный прогон: kernel CUDA event **0,198656 ms**, CUDA context/alloc/kernel/copy host **112,723079 ms**; CPU harness **8,096408 ms** (включает создание emulator и снимки памяти). Метрики несопоставимы как throughput benchmark: разные измерительные оболочки и малый batch; ускорение не заявляется. Код сборки/CSV/log/точный GPU executable сохранены.

Воспроизведение из корня compiler checkout:

```sh
nvcc -O2 -arch=sm_75 reports/cuda-z80-screen/smoke.cu -o /tmp/cuda-z80-smoke
/tmp/cuda-z80-smoke > /tmp/cuda-z80-gpu.csv
cd minzc
go run ../reports/cuda-z80-screen/cpu.go /tmp/cuda-z80-gpu.csv
```

## Реально сохранившиеся таблицы

`historical-inventory.json` перечисляет каждый файл этих двух table каталогов: размер, первые 64 байта/их SHA256; полная SHA для файлов ≤10 MB и отдельно compressed 6v (23,6 MB). Большие IX/JSONL/enriched assets не загружались целиком и не имеют полной SHA в этом ограниченном аудите — это явно отмечено, а не заменено SHA заголовка.

- Z80T version 1 подтверждён первыми 256 декомпрессированными байтами 4v/5v/6v. README описывает nVregs:u8, cost:u16 LE, assignment[n]; FF infeasible. Таблица зависит от **точного детерминированного enumerator order**, полного shape/context binding в самих этих records нет.
- `compressed/exhaustive_4v.bin.zst`: 65 293 B, SHA `43bfc542e22e6c55a0a999a4832fd5ddb57888a5bb7fb4b56328a5ea2f9dd1e5`.
- `compressed/exhaustive_5v.bin.zst`: 8 877 143 B, SHA `7b5ed5d013eb225042cff8af6fe835e60cd05dece9388098c6625a8933ea209a`.
- `compressed/exhaustive_6v_dense.bin.zst`: 23 564 419 B, SHA `7ededa62114b5a80d8575c7602121fe309bdbd4545c75a34634b4b80c810e6a3`.
- `enriched_4v.enr`: ENRT v1, count header 156 506; 4 104 402 B, SHA `7b49113c8a885e4fbe6bf0154ff11fbf88426f0d3daacc7ac019408dd4617a58`.
- `enriched_4v.ofb`: OFB1, count header 156 506; 626 032 B, SHA `8da2e67e1336520b3de89d1b27a85ec9237614ee895fc8c52ce79539ba6e7d45`.

Исторический README заявляет exhaustive/optimal counts/coverage. Здесь подтверждены доступность, format magic/header и контрольные суммы, не оптимальность, shape reconstruction или пригодность assignment для нынешнего MIR/ABI. Перед переносом таблицы нужно независимо проверять текущие interference/width/ISA/ABI constraints, полное покрытие обязательных vregs и emitter compatibility; importer работает над этим отдельно.

## Практический следующий шаг

Сохранившийся evaluator можно использовать для массового **предварительного отбрасывания** arithmetic candidates с явно объявленным modeled-state contract. Перед включением результата — authoritative CPU validation на реальных ASM bytes, all observable live regs/flags, memory/order/alias/wrap/stack и реальный T/BIN. Unsupported instructions/effects должны отказать до запуска; отсутствие opcode нельзя считать NOP. Для повышения скорости полезнее сначала batch kernel в живом CUDA context, затем измерить полноценный end-to-end pipeline, включая compilation/copy и долю прошедших CPU validation. Текущий cold smoke сам по себе не даёт ускорения.

Все новые данные локально в `reports/cuda-z80-screen`, manifest SHA256; копия на `/mnt/safe/minz/grace/2026-10-11/cuda-z80-screen`. Оригинальные большие таблицы уже лежат на безопасной шаре и не дублировались.
