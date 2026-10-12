# Full MIR gate и условные аргументы

Root fullMIR57a7cc36 terminal **PASS80,768с**: extern ABI и typed wordspill stores включены; последующий OpCallCond fix не входит. Scoped HIR memoryhost/profile/ASM gate PASS4,733с; Nanz regex не имел тестов, его0,008с не является проверкой.

Root0da59af4 интегрирует e38a2f9b: Inst.Uses учитывает predicate и Args для условногоCALL. Previously scratch-generation could treat call-only argument dead and zeroBC beforeactualCALL. Native/VM regression actualcallee/globalwrites/PCSP/fullRAM,8boundaryvalues×taken/inverted paths PASS0,022с. Обе ветви иcallcount проверены independently.

Read-only audit: Verify/DSE используют Uses, поэтому fix распространяется наdefinition/usecheck иdeaddefinition retention. Renumber ужеобходитArgs; blockliveness обходитArgs отдельно, не был исходной причиной. Z80 scratch/caller liveness несколько Uses consumers получаютправильныеаргументы. Whole selfparser native proof послеfix собираетсяподагентом; здесьготовностьпакета не заявлена.
