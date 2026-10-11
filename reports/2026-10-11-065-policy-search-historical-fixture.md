# Policy search: тестовый источник совпадает с историческим guard

Три падения `TestMemoryHostPolicySearchPreflightAndExactOriginal`, `Lifecycle`, `OwnedInputsAndSize` объяснялись расхождением тестовых входов: `input.Source` читался из развивающегося `examples/nanz/self_lanz_parser.nanz` (SHA256 763d9f369e3bbbdbcb84dee2627aff5ee90e57d99ad8d0f69e7db0c8325a6b51), а factory уже разбирала исторический архив. Production policy search допускает только исходник SHA256 0e92689c6bb4a92fd681c9bc70d5d50549631a0fdd669cd026f723cf4e197526. Отказ происходил до factory и не проверял предполагаемые lifecycle-инварианты.

Изменён только helper в `memory_host_policy_search_test.go`: source bytes берутся из `docs/grace-parser-prospective-heldout/artifacts/source.nanz` с явной проверкой SHA против production guard. Исходное имя source unit и независимый factory сохранены. Production admission, schema и исторические артефакты не менялись.

Полный lifecycle по-прежнему проверяет восемь измеренных native candidates и 48 fresh factory invocations, отказ мутациям на фазах 25/48, отсутствие совместных HIR-узлов, владение переданными input bytes/workload, strict Speed/Size ranking и отказы поддельным evidence. Это восстановление тестового покрытия закрытого исторического эксперимента, а не разрешение поиска для нового parser source.

Исходный полный pipeline cc502270: FAIL 1649.277 s; этот отчёт не заявляет полного pipeline PASS. Логи и identities: `reports/grace-policy-search-fixture-artifacts/`.

Полная целевая команда `go test ./pkg/pipeline -run '^TestMemoryHostPolicySearch(PreflightAndExactOriginal|Lifecycle|OwnedInputsAndSize)$' -count=1 -timeout=15m -v`: PASS 419.343 s. Lifecycle PASS 295.88 s, OwnedInputsAndSize PASS 115.21 s; все отрицательные контроли сохранены и проходят.
