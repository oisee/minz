# Полный core gate и ещё один исходный iterator

Полный core на a3cc555f завершился exit 0: HIR 23,944 с, MIR2 8,916 с, Nanz 13,170 с, VIR 2,100 с, CLI 384,168 с. Это контекст после guarded recursive runtime, typed_print, callable signatures и исправления MultiCall. Следующие test-only покрытия не меняют production того gate. Full pipeline 22e426de пока live; его более ранний контекст не подменяется этим PASS.

f5372978 интегрирован как 5df343be. Точный архивный ex6_foreach.nanz: sum_chain/max_chain, 60 вызовов и observer, независимо проверены modulo sum/max, ordered reads и весь 514-byte buffer, PC/SP/code; 291930 T / 59 B относятся к полному workload, не одной функции и не оптимизационному выигрышу. Root focused PASS за 0,084 с. Каталог 194 путей содержит 33 bounded VM/native proofs плюс 1 native-only CPM demo; остальные пути имеют явные ограничения/remaining proof. Каталог PASS за 2,038 с.
