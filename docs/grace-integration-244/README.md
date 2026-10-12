# Pascal decimal: root повтор интегрированного исправления

Semantic fix `a24cc755` и bounded runtime admission `8e83f78b` (donors5f95cb1f/a4915bbc): signed Integer и unsigned Word выбирают разные процедуры печати. Before −1→65535/−32768→32768 сохранён; после исправления signed magnitude/prefix совпадает с независимой десятичной моделью.

Root pipeline production full-domain + mutations + существующие CP/M regression: PASS 29,475 с. Проверены все 65 536 signed и unsigned patterns × два BDOS host контракта, всего 262 144 native вызова formatter; fullRAM/code/private stack/guard и точный output. Форматтерный full-domain использует ordinary options; unchanged Hello/Factorial main исполняется во всех трёх режимах отдельно.

Typed frontend selection отдельно PASS; targeted MIR2/CLI PASS. Whole Hello/Factorial COM повтор PASS, точное время в журнале. Private branch labels дополнительно привязаны к фактическим machine PC (`5018b0b9`); отдельный post-followup regression записывается следующим журналом.

Это не VM stdout симуляция и не real hardware. Полный current pipeline/CLI не заменён targeted gates. [Полный reproducible packet](../../reports/pascal-decimal-runtime/README.md).
