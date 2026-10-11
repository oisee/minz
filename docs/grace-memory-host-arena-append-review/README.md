# Независимая проверка arena + append proof

Проверен production source commit `a61fbbb1` (`arena_append_proof.go` SHA256 `b012adcd65b11c0c6dde507a6f69aa7aa500aed3d803f3de36a7ab4baf38f882`). Конкретных оставшихся дыр в ограниченном HIR-контракте не найдено.

Каждая включённая функция проходит прежний строгий standalone-validator на механической проекции только её actual-функции и прикреплённых origins. Затем полный actual-модуль отдельно проверяется: маскируются только уже проверенные включённые тела; metadata, globals, source units и остальные функции сравниваются полностью. Выключенные функции сравниваются без маски. Полное точное объединение origin-путей проверяется после проекций. Исходные две функции обязаны соответствовать прежним точным шаблонам. Storage-exclusion проверен для полных source/actual до проекций. Standalone API не изменялись.

Дополнительный независимый probe: 15 случаев, PASS 1.804s. Отказ для переименованной/дублированной функции, общей statement identity, постороннего origin, переноса origin на новый prefix, globals i/p/j и двух generated base names, лишнего host, а также трансформаций при выключенных флагах. Mask 0 с неизменённым модулем принят. Лог содержит точные причины отказов.

Ограничения: это предкомпиляционный структурный контракт закрытых byte-host эффектов; он не подтверждает gas, внутренние global-read counts, post-lowering lineage, машинный бинарник или прибыль композиции. Actual четыре варианта измеряет другой агент. В этой проверке машинный workload и broad suites не повторялись. Parent и owner production files не менялись.

Воспроизведение: в checkout `a61fbbb1` скопировать `probe.go.txt` в `minzc/pkg/hir/arena_append_independent_review_test.go`, затем из `minzc` выполнить:

```sh
GOCACHE=/tmp/minz-grace-gocache /home/alice/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin/go test ./pkg/hir -run '^TestArenaAppendIndependentReview$' -v
```

Probe использует fixture из tracked `arena_append_proof_test.go`. Он сохранён как артефакт, а не дополнительный production test. Проверка source mutation владельца и исходные четыре позитивных маски были выполнены владельцем отдельно.
