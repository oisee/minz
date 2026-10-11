# Полный MIR2 после PFCCO repair; новый аудит эмулятора

На 8a4d8519 весь MIR2 повторно прошёл за 12,392 с, включая PFCCO body/adapter fix и предыдущие native regressions. Это полный пакет MIR2, не весь pipeline или CLI. Лог сохранён в docs/grace-integration-060 и на шаре.

Read-only аудит подтвердил опасную связь screen SMC hook с данными: запись старого байта DF/E7 вызывает RST input hook, а SetRegisterA пишет в RAMffff вместо A. Ранее отдельный flat-RAM mapInPlace тест явно отключал этот hook; теперь готовится исправление самого эмулятора и независимые actual-store/actual-RST regressions. До их завершения нельзя переносить доказательства произвольных WithScreen workloads на эту границу. Controlled profile runner заменяет SMC tracker на собственную проверку границ; его записи не используют screen hook во время основных вызовов.

PFCCO PBQP opt-in в pipeline и историческая joint-search CLI fixture проверяются отдельными агентами. Полный pipeline-cc502-full остаётся live в исходном процессе, terminal PASS не заявляется. Все прежние артефакты сохранены.
