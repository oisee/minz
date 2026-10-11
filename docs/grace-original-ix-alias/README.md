# Original IX load/store: ordered pointer aliases

Exact `examples/nanz/07_ix_load_store.nanz` не менялся. Полные исходные `sum_with_ix` и `roundtrip_ix` проверены raw VM и фактически собранным Z80. [Language Book v7 §2.6](../Nanz_Language_Book_v7.md#26-pointers-and-arrays) задаёт indexed byte loads/stores; independent host model выполняет исходную запись `bc_ptr[0]=val` перед тремя чтениями. Поэтому aliasing влияет на ожидаемый результат, а oracle не притворяется, что указатели всегда различны.

Шесть persistent layouts: все три указателя различны; HL=BC; DE=BC; HL=DE; все совпадают; BC указывает на byteffff. Каждый layout исполняет `sum_with_ix` и все256val через `roundtrip_ix`: **1536 roundtrips +6sumcalls**. Буфер258байт содержит всеbytevalues и guards; byteffff — отдельная область. Все259байт проверяются после каждого вызова, вместе с uint8sum modulo256 и точными address/value events в порядке write→read→read→read. Guards/неиспользованные хвосты неизменны. Для byteffff не заявлен многобайтовый wrap: source index всегда0.

Native runner проверяет стек, PC/SP возврата, instruction/data admission и запрет обращения к недекларированным областям. Явные integer-address workload drivers вызывают неизменённые pointer entries; отдельный observer нужен host contract и не считается ещё одной программой. Source не имеет main/globals/assertions/import/OS. Проверка не заявляет hardware execution.

Результат PASS: **218 507 T / 38 B** за1542algorithmcalls+observer, SHA256 `ea02b537e95d125161e7de70558db6a2a1e88d7898443a347a4550670866b73e`. Сохранённый ASM действительно содержит `PUSH HL; POP IX; LD (IX+0),A` в roundtrip, затем чтения исходных указателей. Образ38B включает pointer driver и observer; размер не является отдельным optimization comparison.

Catalog сохраняет194paths; теперь34VM/native bounded exactfiles плюс1separate nativeCPM-only. Один sourcefile с двумя entries считается один раз. Никаких compiler/backend/sourcealgorithmedits не потребовалось; miscompile в матрице не найден. Existing foreach/mapInPlace также проходят. Полный медленный pipeline не запускался.

Артефакты содержат exactsource/SHA, workload/driver, raw/nativecapturessnapshots/ordered events, independentmodel, MIR/ASM/BIN и SHAmanifest. Повтор изminzc: `go test ./pkg/pipeline -run '^TestOriginalIXPointerAliasVMZ80$' -count=1 -v`; экспорт `MINZ_IX_ORACLE_ARTIFACTS=/absolute/path`.
