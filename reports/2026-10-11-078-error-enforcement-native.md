# Original ?-named error enforcement: native and VM proof

Полный исходный examples/nanz/15_error_enforcement.nanz заморожен 658273f4 до замера, без урезания функций или переименования. На исходнике сохранены safe_div?, safe_add?, compute, try_div, main. Oracle Book v7 Appendix K CY+A, naming convention ? и точные branches исходника.

Все256×256 пары каждой из четырёх функций: divide0→error1, addition overflow→error2, compute проверяет divide и addition5, try_div передаёт checked division. 262144 вызова на default/GraceSize/GraceSpeed, 786432 total. Для каждого проверены raw/final VM CallWithOutcome (value/failed/code), настоящий assembled Z80 A/CY, исходный returnPC/SP, полный RAM/code кроме ограниченного использованного stack и точных spill bytes. Источник не содержит asserts, его комментарии сохранены.

Полный main печатает `105`, успешно возвращает NC при начальном CY=0 и1; raw/finalVM имеет явный print-u8 decimal host, native использует исходный порт23 runtime с отказом другим портам. Исходник, ASM/BIN, raw/finalMIR, allocation, результаты и stdout сохранены. Это ORG8000 emulator image, не реальная CP/M OS. Дополнительная проверка ordinary CompileHIRWithOptions и CompileHIRSteps запускает их реальные images с исходнымmain и тем же output/NC/RAM/SP oracle (4native runs), не только сравнивает строкиASM.

Terminal native matrix PASS42.309s, both API native parity PASS0.037s. Full MIR2/HIR/Nanz PASS24.873/26.824/14.599s.

Отдельный correctness followup 1f54c1e5: @check/@propagate обязаны иметь0args; @error — один u8 code, literal0..255 (другие widths требуют явного преобразования). Сконструированный malformed HIR семи видов проверен через оба публичных API: аргументы нельзя молча отбросить. MIR Verify сначала проверяет nil module/function/block/instruction и возвращает диагностику, не панику из нового ABI scan. Эти fixes не расширяют CY ABI на word/tuple/extern/indirect/other backend; ограничения отчёта077 сохраняются.

Повтор: cd minzc; MINZ_ERROR_ENFORCEMENT_ARTIFACTS=<dir> go test ./pkg/pipeline -run '^TestOriginalErrorEnforcement' -v; go test ./pkg/pipeline ./pkg/mir2 -run '^Test(ErrorIntrinsicArguments|VerifyRejectsNil)' -v. Замороженная матрица в calls.json.gz; exact payloads в manifest.json, tracked+SHA verified. Это полное bounded покрытие всех named entries данного исходника, не all-corpus claim.
