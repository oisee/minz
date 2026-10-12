# Сохранённый каталог: актуальность всех evidence SHA

Перегенерирован catalog338 через TestShowcaseExecutableOracleCatalog после нового ZX full-oracle routing/cleanup. Прежний JSON сохранял старый SHA original_zx_apps_test.go, хотя reviewed-updates уже обновлён. Теперь проверены все194source hashes и647reviewed file references; SHA совпадают с текущими файлами. Ссылки file::TestName проверяются по файлу; имя теста дополнительно обрабатывается существующим exporter.

Числа категорий не изменились:153independent/bounded VMnative,21nativeASM,10CPM,2SMC,5Agon,2pending,1hostZVM. Хеш файла — provenance, не повтор выполнения каждого workload: известные native результаты и границы остаются своими датированными checkpoints. Runtime/source behavior не менялось. Полный pipelinedd100284 ещё выполняется.
