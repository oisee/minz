# Function-pointer consumer: доказательство и каталог

Исходный 09_function_pointers.nanz unchanged. На всех 256 байтов raw/final VM выполняют apply(double/triple,x) с правильным symbolic function address и независимым modulo256 oracle. Native apply(triple,x) проверен до/после checked table: 123 → 111 T, модуль 34 → 31 B, SP восстановлен и полный compiled image неизменён. Генерация требует явного сохранённого Z3; focused test PASS за 0,408 с. Native double indirect и whole arbitrary callback application не заявляются.

Каталог содержит 194 точных пути: 32 с ограниченным independent VM/native evidence и 1 CPM native-only runtime demo. Новая foreach ветвь интегрируется отдельно; исторические catalog snapshots сохранены.
