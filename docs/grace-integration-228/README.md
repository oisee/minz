# Текущий CLI сохраняет именно проверенный COM

Собран compiler checkpoint c8f7e434. Public CLI трижды собрал неизменённый CP/M impl demo с runtime cpm и stack1024: ordinary, Grace Speed, Grace Size. Все три COM побайтно равны соответствующим root native-tested program.com из пакета221:1419 байт и SHA199d968289be627478633709f5130b9113cea6f7a1244d4062e16746b8b1b46d.

Это проверка actual CLI output после новых Nanz/CPM changes, не новый аппаратный запуск. Команды, source/compiler SHA и stdout сохранены в results.json. Native runtime behavior доказан отдельно в221, поэтому равенство здесь привязывает CLI к исполненному артефакту.
