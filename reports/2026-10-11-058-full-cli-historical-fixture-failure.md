# Полный CLI-прогон: одна историческая fixture несовместима

Прогон на 37475dff завершился: весь MIR2 PASS 7,264 с; cmd/minzc FAIL 127,568 с. Единственное падение — TestSearchCLICollectAndPairedReplay: exact original SelfLanz source required. Тест копирует текущий examples/nanz/self_lanz_parser.nanz, тогда как узкая legacy joint-search CLI ожидает исторический исходник SHA 0e92689c... до исправления CString main. Внешний admission source guard не ослабляется. Привязка исторического теста к immutable snapshot исправляется отдельным коммитом; результаты production correctness не подменяются.

Полный лог сохранён в docs/grace-integration-058 и на /mnt/safe/minz/grace/2026-10-11/compiler-docs/. Большой pipeline остаётся live в прежнем процессе. Полный текущий CLI PASS пока не заявляется.
