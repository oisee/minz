# Совместные варианты и обычная caller workload

Следующий этап Grace PGO проверяет весь выбранный набор преобразований на actual assembled Z80. Стоимости независимых функций не складываются как доказательство выгоды комбинации.

Input Nanz → controlled VM profile с source/executable/workload compatibility → перечисление observed eligible sites → fresh HIR на каждый набор → обычная legality Grace → raw MIR2 VM oracle → production pipeline/PBQP → assembler → persistent Z80 workload → выбор по total T-states или module bytes.

`MeasureProfileVariantSets` перебирает максимум четыре наблюдавшихся eligible sum site, до 16 наборов. Каждая комбинация требует совпадения results/global/resource bytes с исходным controlled run. Oracle повторно исполняет заявленную workload и проверяет counts/guards против загруженного профиля. All-original выигрывает равные цены. Эта проверка относится к конкретной workload и inputs; не является доказательством всей программы на всех inputs.

`RunZ80ProfileWorkload` исполняет обычные внутренние calls: bootstrap нужен только для входа в каждую объявленную invocation. Программа загружается один раз, RAM/globals/resources сохраняются между invocations; resource pointers разрешаются в реальные target addresses. Стоимость invocation — вход→RET, включая все внутренние calls и исполнение callee; внешний bootstrap исключён. Бюджет MaxSteps учитывает инструкции bootstrap и всех invocation суммарно.

CLI запуск:

```sh
mz caller.nanz --hir-profile-auto /tmp/caller-pgo \
  --hir-profile-joint --hir-profile-auto-workload workload.json \
  --asserts=none -o /tmp/caller-pgo-winner.bin
```

Joint evidence хранит все кандидаты, exact binaries/SHA256, caller costs, results/финальную память, совместимый input профиль и chosen index. Emitted binary берётся из measured winner. Размер whole-module; это reusable compiler fragment, entrypoint/bootstrap полноценной standalone target программы пока отдельный уровень.

Ограничения controlled target harness описываются реализацией и tests: pointer-valued returns и pointer-containing storage требуют нормализации адресов; ASM/ports/external hosts/SMC не должны молча получать VM-equivalence claim. Memory layout обязан исключать code/resource/bootstrap/stack overlaps.

## Исполненные результаты

Caller с двумя функциями (small_sum длина1, big_sum длина5), двумя ordered invocation, globals data/output: результаты 16/16 и память совпадают на исходном raw VM и всех assembled candidates. CLI собран один раз, выполняет весь маршрут:

| Вариант | Module bytes | Caller T-states (2 invocation) |
|---|---:|---:|
| обе original | 109 | 2234 |
| small original / big pointer | 158 | 1970 |
| small pointer / big original | 158 | 2386 |
| обе pointer | 207 | 2122 |

Speed выбирает только big pointer: **−11.82% caller T-states**, +49 bytes. Размер выбирает обе original. SHA256/binary selected совпадает с фактически измеренным кандидатом. Порядок кандидатов в evidence следует отсортированным opaque SiteIDs и может отличаться между source paths.

Другой caller, вызывающий два sum длины5 с symbolic resource pointer, проверяет выгодность joint rewrite: 3476/3212/3212/2948 T-states и107/156/156/205 bytes, best обе pointer. Этот результат получен совместным исполнением, независимые модели его не заменяют.

Дополнительно test workload проверяет persistent mutations resources/globals, void caller и u16 return. Он обнаружил существующий Z80 wrong code: constant load не сбрасывал alias C↔A, `tally+1` использовал прежний input byte и давал25 вместо11. Исправление codegen инвалидирует alias при записи constant; five ALU regression cases и исходная persistent workload проходят.

Exact sum cost domain усилен memory heatmap: каждый input byte читается ровно один раз, writes0, zero-trip нет data reads, другие addresses разрешены только code/ABI stack. Mutant с лишним zero-trip read отвергается. Heatmap не является полной ISA memory proof и не отслеживает порядок чтений в общем caller.
