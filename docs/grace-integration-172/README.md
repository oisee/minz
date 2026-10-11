# Интеграция 172: corpus и профильная граница после struct ABI

Полный TestShowcaseCompileAssemble на30c2efda: **PASS9,358с**,193PASS/1прежнийcapacitySKIP. Включены callerbuffers/localcopies/nestedinlinecopies и точныеmetadata. Это compile/assemble, не исполнение всех194programs.

Новые profile boundary regressions **PASS0,068с**: scalar caller read(17) вызывает make()->Pair и контролируемый nativeworkload даёт18; прямой make aggregateentry получает явный отказ в v1, а не сравнение адреса с objectbytes. Это подтверждение текущей границыschema, не замена будущего object oracle. Fullpipelinecfaa7777 и fullCLI44d8952b остаютсяrunning по конкретнымhandles.
