# Смена измеренного выбора и отказы

Добавлены artifact-only проверки тойже опубликованной CLI1bd955 и исходного
SelfLanz. Три двухвызовныхworkload наблюдают обе exactsourceпетли; append
BodyEntries=0, но Entries=1. На каждом реально измерены четыре варианта
под speed иsize, всего24candidatepayload и6successfulCLIcommands.

| Workload | Original T | Arena T | Append T | Both T |
| --- | ---: | ---: | ---: | ---: |
| Zero body | 784 | 763 | 948 | 927 |
| First equal | 1672 | 1587 | 1836 | 1751 |
| First mismatch | 1608 | 1564 | 1772 | 1728 |

Speed выбираетarena(mask1), sizeappend(mask2) во всехтрёх. Это реальная
смена выбора по сравнению с четырьмя большимиcorpora, где speedmask3.
Нулевое тело append не оправдывает hoist по времени; code-size цель отдельна.
Бинарныеразмеры original/arena/append/both2091/2212/2076/2197B.

ПятьnegativeCLIcases далиexit1 и не создалиoutputdirectory:
unobservedappend, source-tail undeclaredreadC001, destination-tail
undeclaredwrite9002, readonlyheader9000write, gaslimit1. Недостоверные
профили не даютacceptedselection илиselectedbinary. Hostdiagnostics
сохраняются отдельно: source-tailprefix3successfulevents, destination-tail
prefix4, readonlyheader0. Gas — infrastructureFailure=nil, не семантический
hostdenial. Независимый sourceoracle не моделирует instructiongas.

Все8rawprofilediagnostics прошлиfullstate/orderedbyteevents/11sourcecounters
проверку независимой originalsource-моделью, включаяpartialfailedprofiles.
Стоимость privateprofilehooks не используется как nativeT илиlegality.
Native24candidate/6selection/5refusalabsence audits прошли независимыйreview;
productioncode/contracts не меняются.

[Артефакты](grace-memory-host-choice-artifacts/manifest.json).

NegativeCLI проверки использовали внутреннийcollector; suppliedfailedprofile
route не запускался отдельно. Его отказ вloader не заявляется как новый
проверенныйCLIcase.
