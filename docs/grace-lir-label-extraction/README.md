# Первое извлечение зависимости LIR: ASM-метки

База `ea4039e7`. Точная прежняя функция `lir.SanitizeAsmLabel` перенесена в
нейтральный `pkg/asmutil.SanitizeZ80Label`. Production pipeline использует её
для extern stubs; VIR — для module globals. Research LIR API временно оставлен
тонким wrapper. VIR больше не импортирует LIR прямо или транзитивно: сохранён
результат `go list -deps ./pkg/vir`.

Семантика имён сохранена: четыре замены `@ . - $`, case-insensitive защита имён
регистров через `v_`, остальные символы без изменения. Это извлечение utility,
не новая полноценная политика проверки/нормализации символов. Native MIR
sanitizeIdent пока не изменён, поскольку имеет другой контракт.

Сохранённый `tests.txt` подтверждает:

- reserved-name/punctuation fixtures и неизменность апострофов/Unicode;
- extern stubs с точным собранным BIN `cd0780cd0880c9c9c9`;
- VIR globals с точным BIN `493412` и адресами меток;
- existing disabled-LIR whole-module tests, где production PBQP ASM
  сравнивается с обычным вариантом;
- existing research LIR codegen fixture.

Команда: Go 1.26.8, `GOCACHE=/tmp/minz-grace-gocache`:

```
go test ./pkg/asmutil ./pkg/pipeline ./pkg/vir ./pkg/lir -run '^(TestSanitizeZ80Label|TestExternStubSharedLabels|TestModuleGlobalsSharedLabels|TestLIRDisabledWholeModule|TestPipeline_LIRCodegen)$' -count=1 -v
go list -deps ./pkg/vir
```

Первый результат команды PASS для всех четырёх пакетов. Поиск `pkg/lir` в
списке VIR dependencies имеет ожидаемый статус 1 (совпадений нет).
Convergence (`LIRCheck`, `LIRResults`) остаётся до следующего отдельного этапа;
Options, profile IDs, allocator и генерируемые инструкции не менялись.
