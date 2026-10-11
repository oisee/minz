# Исходные frontend, GCD и typed showcase: актуальный каталог

Сохранены все 194 точных пути. После независимых root-повторов каталог содержит 97 independent/bounded и 40 pending-путей; остальные ASM/runtime/host ограничения сохраняются. Шесть assert-путей, шесть точных abs-diff/GCD путей и пять enum/alias/value-pipe источников получили явно ограниченные доказательства. Дубли повышены только после отдельной проверки равенства SHA исходников.

Root assert/GCD PASS25,601с; typed showcase PASS,точныйлогрядом. Все119/73/120manifestpayloadсоответствующихпакетов провереныlocal/canonicalshare. Fullbyte/u16 unary coverage иboundedwordpairs/zeroGCD указаныраздельно. Лямбдаex33проверена raw отдельно,final/nativeподтверждают исходныйсуммирующийциклпослевстраивания.

FullCLI11105fb3 иfullpipelinecfaa7777 ещё выполняются. Статусы каталога не доказывают overallcurrentfullsuitePASS; общая цель остаётся активной.
