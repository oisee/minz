# Интеграция 171: сводка текущего ABI и новый full CLI

Верхняя сводка переписана вокруг текущего code state: caller-owned results, local struct value copies, nested inline copying и точные owned signature metadata. Исторические checkpoints не объединяются в мнимый fullcurrentPASS. Прежние corpus/heldout/CLI evidence сохранены.

Новый полный CLI послеABI запущен с заранее сохранённым executable44d8952b (productioncodeafca2aa5); previousfullCLI568b7d08 относится к состоянию доbuffers. Полныйpipelinecfaa7777 продолжает работать по тому же retainedhandle; logпоследниеобновлениянепоказываютreportedFAIL, но отсутствиеFAIL не означаетPASS. Общая цель остаётся активной.
