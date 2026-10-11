# Profile evidence: retained executable follow-up

Исходный evidence из `f09aa753` имеет compiler_id `executable-sha256:b9e43a7268a76ebf2aff99e905300327a065f4971d420044813f027f1dceaa21`. Тот временный go-test executable удалён после завершения; среди surviving /tmp/go-build executables совпадения нет. Старый evidence сохранён неизменённым, наличие его executable не заявляется.

Отдельно создан и сохранён durable `pipeline.test` из branch HEAD `4f729870` через `go test -c`. SHA256 `107c9f003624a273f139fede4747ffdfc389ead52c482603ead5cabe69779a42`. Он запущен из retained operation directory с `-test.run '^TestProfileRealSourceCorpusPopcount$' -test.v`; source resolver использует реальный файл относительно этого каталога. Новая evidence хранится в отдельном `profile-global-retained-executable/evidence`, не заменяя старый packet.

Новый retained executable/hash совпадает с compiler_id evidence. Все 256 inputs снова PASS: 7168 T-states, полный module BIN 512 B, независимый popcount oracle. Exact executable, исходник/workload/profile/evidence/results, baseline/selected ASM/BIN и manifest всех payloads сохранены вместе. Это replay собственно теста генерации evidence, не новый profile loader/replay протокол.

Лог: `profile-global-retained-executable.txt`. Payloads: `artifacts/profile-global-retained-executable/manifest.json`.
