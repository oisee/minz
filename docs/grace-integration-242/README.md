# Свежий повтор заранее отложенных нагрузок

На `13d9c298` запущены четыре неизменных frozen эксперимента ParserProspectiveHeldout, HashmapMeasuredTrainingAndUnseenWorkload, PFCCOProspectiveHeldout и PascalBubbleSortProspectiveHeldout (session5353). Дополнительно запущен ArithmeticProspectiveHeldout со своим исходным freeze. Новые input freeze не создаются, известные входы не переименованы в prospective.

Этот повтор следует за TermCondRet/Pascal Hello изменениями. Десятичное signed/unsigned Pascal исправление ещё в отдельной ветке и данным запуском не проверяется. Полный core76945771 уже PASS; retained полный pipeline29df08bb продолжает исполняться на прежнем executable.

Terminal результаты будут сохранены отдельными журналами; сейчас это запись начатых проверок, не утверждение об их PASS или о завершении цели. Arithmetic может корректно выбрать исходный вариант: отсутствие улучшения не скрывается.
