# Интеграция 174: поддерживаемые пакеты собираются без LIR

На0df54c97 после caller-owned ABI `go test ./pkg/... ./cmd/... -run '^$'` **PASS**: все поддерживаемые package/command targets компилируются (тестовые тела не исполнялись). `TestRetiredLIRConfigurationRefused` отдельноPASS: legacyUseLIR/LIRCheck конфигурация не принимается молча. В tracked tree minzc/pkg/lir отсутствует, ordinarypipeline/CLI не имеют LIR переключателя или импорта. Два pkg/lir упоминания — исторические комментарии об извлечении shared sexpr/datalog, конфигурационные строки остаются только negative test.

Это поддерживаемая package/command сборка, не `go test ./...`: исторические scripts mixed-main/relative-import проблемы отдельно сохранены в147. Fullpipelinecfaa7777/fullCLI44d8952b остаютсяrunning; CLI уже сообщил две expected-refusal несовместимости, независимыйreviewer проверяетnativefixtures прежде обновленияtestexpectations.

Следующий корпус originalLizp self_tokenizer frozenb9e93e34: nativewholemain/persistentstate/output oracle в работе отдельногоагента. Tokencomment skipped, sourceoutput134425newline, не пробелы из комментария.
