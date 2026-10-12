# Повтор legacy import и отрицательного аудита таблиц

Compiler fd760106. TestActualLegacyCandidateImportCorpus и TestCheckedTableNegativeCorpusAudit: PASS, 4.949 s. Эти opt-in тесты были пропущены в полном pipeline350149ad; результат дополняет его.

Legacy native/GPU JSON и сохранённый bounded ENRT sample заново проверяются для текущих ops/ABI. Отрицательный аудит исследует12 реальных программ и сохраняет причины отказа bridge, historical hits, допущенные назначения и сравнение размера.

Новый solver допускает8 leaf-кандидатов: weighted36→12B и triple7→4B меньше baseline; add,double,hash3,test_eq,test_neq,low_nibble равны по размеру. При равенстве baseline сохраняется. Большинство функций не соответствует текущему single-block integer leaf bridge; это ограничение bridge, не доказательство отсутствия полезного назначения.

Независимое native покрытие подходящих кандидатов находится в отдельных offline-table и indexed-coverage отчётах. Сам этот диагностический аудит не считается новым native-доказательством всего корпуса. Все исходники, tables, ops и результаты сохранены здесь с SHA256.
