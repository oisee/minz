# Карта корпуса и conditional-call guard

Интегрирован8bfb0b02: каталог193 сверяется с существующими независимыми тестами,
а не только с недавними дополнениями.22 точных source paths имеют названные
bounded workloads VM+assembledZ80.42 paths лишь упомянуты Go-тестами; ссылка
не считается behavioralPASS. Derived CString CP/M main отдельно связан с
фактическим savedCOM/fullmemorycapture и не объявляется исполнением originalmain.

Новая original30_stream_test нагрузка:5 capacities0/1/3/63/64 ×70 writes.
Независимый model проверяет64-bytebuffer,6-bytedescriptor, dropped writes,
pos/cap, relocatedpointer и balancedreturnSP. RootrepeatPASS: Nanz1.262s,
pipeline0.213s. Originalfunctions сохранены; stdout/stdlibStream не покрыты.

Ревью pure addresshoist выявило пропущенный OpCallCond; eb3bfb88 добавляет
запрет. Все direct/indirect/conditionalcalls сохраняют loopIR без изменений;
rooteffectregressionsPASS. Broadcallhoist ранее дал измеренную регрессию,
поэтому запрет имеет практическое основание, а не формальную видимость.

[Карта](../docs/grace-native-coverage-map/README.md),
[rootrepeat](../docs/grace-native-coverage-map-integrated.txt),
[callguard](../docs/grace-hoist-call-rejection-integrated.txt).
20 файлов пакета/guard скопированы и SHA256 проверены на шаре,
integration-038-sha256.json. Общий pipeline2903 ещё выполняется.

Следующие реальные задачи: originalself_tokenizer независимый fullmemorymodel,
third-family prospectiveselection на tailrecursive corpus, boundaryaudit
таблиц/hardconstraints/costs/hostsolverdeadline. Цель остаётся активной.
