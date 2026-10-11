# Production pipeline больше не зависит от LIR

Интегрирован `c3864cf2`: удалены импорт LIR из production pipeline, поля LIRCheck/UseLIR/LIRResults и no-op флаги CLI --lir/--z3. Native expected-value оракулы сохранены; прямые research проверки пока остаются в пяти тестовых потребителях.

Изменение Options меняет configuration ID: старые профили явно отклоняются, исторические evidence IDs не переписываются. На интегрированной ветке проходят CLI отказ от снятых флагов, отказ старой конфигурации и MultiFunctionProfileVMZ80 со stale/duplicate/missing отрицательными случаями.

Обычный `go test ./... -run '^$'` завершился FAIL из-за каталога scripts: смешанные packages и старые относительные imports. Это выявленные ограничения общего build gate; успешные отдельные пакеты не превращают весь запуск в PASS. Отдельно проверяется build gate cmd/pkg/tests. Сам пакет LIR ещё существует; перенос полезных тестов и полное удаление выполняются следующим шагом.

Подробные проверки и сохранённые native оракулы: `docs/grace-lir-convergence-removal/`. Исправление signed abs_diff и быстрые native word multiply10/12 идут параллельно.
