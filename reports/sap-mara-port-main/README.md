# SAP MARA: исходный main с портовым SQLite

Неизменённый `examples/nanz/sap_mara_cpm.nanz` исполняет исходные SQL/TUI ASM-тела. Настоящая SQLite создаёт mara/makt, получает LEFT JOIN с фильтром FERT и сортировкой MATNR. Независимо заморожены три строки: 100-100/Pump Assembly GRUNDFOS, 100-110/Motor 3-Phase 5kW и 200-100/Desktop PC Standard.

Три режима и два BDOS clobber-контракта прошли вместе с повтором sqlite_test: **PASS 0,408 с**. SAP даёт ровно 360 байт stdout, 31 команду и 12 column_text значений. Все записи строк в SQL buffer, включая NUL и сохранённые хвосты, сверены по адресам/байтам. Полный RAM проверен кроме точных spill и использованного стека. Query/database закрыты; PC=7003/SP=F000 восстановлены.

Все режимы дают **2820 байт / 78 892 такта**. Время SQLite/BDOS callbacks не включено. Grace выигрыша нет. Стенд проверяет порядок типов SQL команд и итоговые данные; полный byte trace портов сохраняется, но отдельного независимого оракула для каждого байта trace пока нет.

Это native тест с test-owned SQLite/BDOS, не raw/final VM ASM emulation и не физический CP/M/provider. [Лог](root.txt) · [Freeze](frozen/freeze.json).
