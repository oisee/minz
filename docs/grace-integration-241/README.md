# Self-parser: чистые классификаторы в каталоге

194 пути и прежние категории сохранены. Исходный self_parser получает дополнительные exact-source доказательства четырёх чистых классификаторов на всех 256 байтах в трёх режимах. Full module сохранён; native PC/SP/fullRAM и execution closure проверены, raw/final VM memory effects запрещены.

External-runtime-needs-audit сохранён: файловые функции, печать и основной parser main не исполнены. Исходная 516,227-секундная проверка и ускоренный 2,943-секундный стенд сохранены отдельно; выигрыш относится к стенду, не к программе. [Полный пакет](../../reports/self-parser-classifiers/README.md).

Каталог проверяет SHA исходника, oracle test и frozen evidence, а не повторно исполняет весь корпус. Точный focused terminal результат в catalog-test.txt.
