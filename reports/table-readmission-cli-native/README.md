# Полный CLI → native Z80 replay таблицы

Новый TestReadmissionCLIWeightedNative собирает реальные importer и compiler, подготавливает table из saved weighted proposals, компилирует исходный caller-harness с content pin и ассемблирует продукт. All256inputs выполняются настоящим Z80 без host hooks. Независимая формула byte(11*x+1), точный PC возврата и SP проверены. Полный BIN20байт. Мутация table→[] при прежнем pin отвергается без output ASM. Full cmd/import-enrt PASS1,324с.

Первый диагностический запуск нового стенда ошибочно подавал input вA; фактический caller ABI — B→A. Ошибка стенда сохранена в harness-before.txt, bootstrap исправлен на LD B,x и контракт явно проверяется в ASM summary до исполнения. Backend не менялся.

Артефакты реальной preparation/compile/assemble цепочки сохранены в соседнем table-readmission-cli/executed; здесь дополнительно сохранён replay тест. Ни Z3, ни ENRT не требуются. Общий full pipeline продолжает работать, этот gate не заменяет его terminal результат.
