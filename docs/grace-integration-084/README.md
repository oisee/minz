# Интеграция084: tokenizer PFCCO tradeoff

Report070 интегрирован69987b30. Root удалил ненужный shared /tmp debug output; повтор tokenizer+arena PASS1,837s. Production/admission/default не менялись.

Оригинальный tokenizer ordinaryBIN5474→5462B, fullfixedsequence319166→319442T. Keyword rendering21896→22172T; identifier control18512T unchanged. Все model globals/8192string/4096token/4096source bytes, native stdout, code/literal immutability except exact mutable globals/spills и SP/canaries проверены. Rendering имеет independent hostmodel/native proof; VM output не объявляется оракулом. Same4Badapter retained bothsides, executable5478→5466B.

StrictSpeed должен сохранитьgreedy, Size предпочестьPBQP. Текущий selector ещёнеперебирает стратегии; новый versionedoff/greedy/pbqp API разрабатывается отдельно для поддерживаемогоProfileWorkload domain. Tokenizermemory/output в этомdomain покаunsupported, старыйmode4/replay неменяется.

Пакетreports/artifacts/pfcco-tokenizer иreport070 copiedbyteverified наdatedsharecompiler-reports, SHAmanifest проверен. Полныйpipeline3e143f9f живёт в исходномsession71745 и не включает этот новыйtest; последнийfullcore9b1637ef PASS сохранён082. Цель активна: measured benefit переносов и coverage корпуса ещё расширяются.
