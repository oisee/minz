# Интеграция 186: root полный исходный iterator replay

Root `TestOriginalIteratorLUTCorpusComplete` на46ced4a1 завершился **PASS82,164с**. Это повтор всех заранее объявленных исходных LUT/sum/max/map byte/length нагрузок raw VM, final VM и Z80, включая orderedmemoryaccess и fullRAM/SP. Короткие branch-edge controls и контрпример также прошли отдельно. Все77manifestpayload проверены локально/наcanonicalshare в185.

Этот focused corpus gate не заменяет полный pipeline. Долгий pipelinecfaa7777 ещё выполняется и теперь сообщил также TestMeasureLoopCostModel stalecoefficients; актуальное независимое измерение соответствует191T/originaliteration и другим новым значениям, их обновление проверяется отдельным агентом. До окончательного результата fullpipelinePASS не заявляется.
