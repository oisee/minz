# Exact original array forEach: sum/max and ordered reads

Проверен точный `reports/showcase-src/2026-03-10/ex6_foreach.nanz`: полные исходные `sum_chain` и `max_chain`, изменяющие captured accumulator. Source/SHA не менялись. Это не повторное доказательство `01_sum_array`, а отдельный ранее pending exact path. Файл не имеет imports, globals, assertions, main или OS runtime.

[Language Book v7 §5.1/5.3](../Nanz_Language_Book_v7.md#chapter-5-iterator-chains) задаёт `forEach` и изменяемый closure capture. [Глава 6](../Nanz_Language_Book_v7.md#chapter-6-for-loops-and-ranges) различает array scan и range countdown. Здесь массив обходится по возрастающим адресам; oracle не применяет countdown range семантику. Независимый Go model вычисляет сумму modulo256 или unsigned max и отдельно ожидает точные byte reads.

Матрица: 2 original entry ×5 offsets (1,2,127,128,255) ×6 lengths (0,1,2,127,128,255) = **60 вызовов**. Буфер514байт содержит перестановку всех byte values, overflow и guard endpoints. После каждого вызова проверяются результат, все514байт/неизменённые хвосты, точная последовательность reads с адресами/значениями, отсутствие writes/globals/failure. Raw MIR VM и фактически собранный Z80 оба проходят независимый oracle. Native runner также контролирует PC/SP возврата, stack bounds, code/data доступ и запрещает undeclared effects.

Явные test-only integer-address drivers преобразуют u16 в ^u8 и вызывают полный исходный entry; bounded memory-host требует integer invocation. Отдельный read-only observer нужен host-контракту, его один дополнительный вызов не считается вызовом алгоритма или новой программой. Source алгоритм не копируется и не сокращается. Нет утверждения о physical hardware/OS. Miscompile в этой матрице не обнаружен, production compiler не менялся.

Результат: PASS, **291 930 T / 59 B** за60algorithmcalls+observer; SHA256 `df200ed1899721eb3147d47be45bdaf740ea99803a70a21c51d4b369d93944c1`. Артефакты: точный source/driver/workload, raw/native полный capture, independent model, ASM/BIN/MIR, SHA manifest. Initial checks.txt содержит устаревший текстовый count120; это typo logger, workload явно61invocations=60+observer; исправленный focused.txt верен.

Каталог сохраняет194paths и все ранее доказанные boundaries:32VM/native bounded files плюс1отдельный nativeCPM-only. Driver не расширяет число файлов. Existing map/filter/capture regressions также PASS; fullslowpipeline не запускался.

Повтор из minzc: `go test ./pkg/pipeline -run '^TestOriginalForeachReadOrderVMZ80$' -count=1 -v`; экспорт `MINZ_FOREACH_ORACLE_ARTIFACTS=/absolute/path`.
