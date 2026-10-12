# SQLite и два экранных main: общая ветка

Source checkpoint `7c4ce479`. Root joint gate **PASS 1,013 с**, все три test families исполнились без export-переменных. SQLite whole main проходит три режима и два BDOS clobber-контракта с настоящим SQLite: точные 59 байт вывода, 11 команд, полный RAM и PC/SP. Bounded API дополнительно проверяет send/recv и результаты. Обе неизменённые экранные программы проходят raw/final VM с явными ASM host bindings и assembled Z80 во всех трёх режимах: точные 422/236 байт вывода, одно чтение, RAM и возврат.

[SQLite: доказательства и границы](../../reports/sqlite-whole-main/README.md) · [Экраны: модель и native](../../reports/cpm-screen-main/REPORT.md).

Это test-owned runtime. Публичный CP/M admission не доказан этим запуском. MZX SQLite provider имеет отдельно обнаруженное расхождение handle/status протокола; успех адаптера не переносится на него. Общий полный pipeline на старом retained executable продолжает работать с прежним числовым parser reference; этот focused PASS его не заменяет.

Каталог194путей повторно PASS 2,998 с. Два screen пути теперь bounded independent VM/native; SQLite — native ASM bounded. Все границы сохранены в reviewed-updates и каталоге.
