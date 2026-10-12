# VIR: явное размещение результата возврата

При наличии research Z3 TestVIR_Assert_MultiReturn давал abs_val(5,0)=0 вместо5. Standalone solver помещал x в B/L, а bridge пропускал return move по ABI-классу исходного параметра. После adapter A→L,C→A ветвь return x делала RET без восстановления A.

Bridge теперь всегда добавляет solver-constrained move результата в A/HL. Ни block parameter, ни исходный ABI class, ни прежняя ALU definition не доказывают его фактическое расположение у терминатора. Source hint оставлен свободным, чтобы standalone solver мог переназначать входы. Unit regression и исходный MultiReturn VM/native PASS. Полное assert-family: 16 PASS, 2 FAIL (см. точные counts в логе); GCD и Lookup всё ещё дают неправильные native результаты и не объявлены исправленными. GCD идёт через PBQP fallback; Lookup solver смешивает значения loop-header. Они требуют отдельных исправлений.

Команда: PATH=<research-z3-bin>:$PATH VIR_ASSERT_EXPORT=<output> go test ./pkg/vir -run '^TestVIR_Assert_' -count=1 -timeout=10m -v. Exit1 из-за двух оставшихся ошибок. Production pipeline не использует этот VIR solver; полный production gate продолжает старый immutable executable. Исходники/MIR/ASM/allocations контрпримеров сохранены. Полный Z3-enabled VIR gate, запущенный до изменения bridge, ещё выполняется session88197.
