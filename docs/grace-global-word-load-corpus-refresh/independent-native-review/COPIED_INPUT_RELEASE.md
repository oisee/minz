# Copied input release86e01e0a

Новые четыре workload JSON побайтно равны прежним saved CLI workload.json дляобеихgoals, SHAe234/32bd/f31e/a38d exact. Отличие отпервоначально authored fixtures — только завершающийnewline: новые savedCLI copies на1byteкороче. Typedobject полностьюсовпадает; это честноотмечено, неinputsemanticdrift. Initialreadcomparisonagainstauthoredbytes refused equality untilcorrectreferencechosen; ничего неусреднялось инепересериализовывалось.

Readonlyrelease owner8root-authorized actualCLI runs on saved dba executable. Exactsource/contexts проверены; noCPUreviewer. Пока measurements неterminal, productioncomplete/newwinnerclaims неделаются.
