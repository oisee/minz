# Независимый аудит нового CP/M допуска и IRC строк

На checkpoint686d9e81 запущен повтор четырёх frozen heldout семей; результат ожидается. Свежий полный CLI остаётся live, старый pipeline29df08bb также live. Новые найденные проблемы расследуются отдельно, не подменяют исторические PASS.

CP/M review воспроизвёл accepted MIR форму: local pointer копируется через OpMove с byte destination, а provenance ошибочно считает результат полноценным адресом. При byte spill home код может писать по постороннему адресу. Это конкретная admission ошибка; ordinary source miscompile пока не заявляется. Независимый агент сохраняет exact MIR/allocation/native beforeproof и исправление typed provenance.

IRC classifier raw VM для PING возвращает Unknown: source передаёт обычный length-prefixed string в helper, читающий NUL-terminated bytes. По Language Book это ошибка representation исходника, а не повод менять default string semantics. До изменения sourceSHA заморожен; исправление на c-string literals будет иметь отдельную identity и runtime ограничения. Network/TUI main этим не проверяется.

Общая цель остаётся открытой; эти проверки движутся независимо от двух незавершённых language/runtime решений Lizp и PL/M.
