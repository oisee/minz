# Original CP/M impl: рабочий runtime и root replay

Исходный 51_impl_cpm_demo.nanz сохранён целиком. Рабочий CP/M runtime теперь допускает точный BDOS2 byte adapter с полным clobber-контрактом и доказанные entry-local объекты. Допуск остаётся ограниченным: произвольный ASM, escaping local pointers, динамические/циклические allocations и неподтверждённые адресные stores отвергаются.

Root original/admission/native suite PASS 6,440 с. Исходный main исполняется в трёх режимах и под двумя BDOS2 контрактами, включая полный primary-register clobber; печатает C:75\r\nR:50\r\n, сохраняет память и стек, возвращается в ОС. Новые отрицательные варианты проверяют отказ. CPM MIR stack/runtime verifier suite root PASS 0,229 с.

Agent public CLI выдаёт byte-identical COM всех трёх режимов:1419 байт, SHA199d968289be627478633709f5130b9113cea6f7a1244d4062e16746b8b1b46d. Все36 tracked packet payloads и два retained executables повторно проверены root на шаре. Native main7442 CPU T, stack19 байт; synthetic BDOS service time не включён. Настоящее железо не проверялось.

Исторический runtime отказ и тестовый loader сохранены отдельно; их прежний статус не переписан. Повтор существующих CP/M parser/tokenizer и live-value tests root PASS 2,027 с. Старый полный pipeline29df08bb всё ещё live и предшествует новому runtime.
