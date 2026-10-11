# Каталог: положительная проверка методов без подмены runtime

Все 194 пути и прежние counts сохранены. У `51_impl_cpm_demo.nanz` добавлены exact-source доказательства двух чистых методов и фактического root replay: 394 752 native вызова в трёх режимах. Общий статус остаётся inline-asm-target-needs-audit: main и BDOS ещё не исполнялись. Metadata проверка каталога PASS 2,385 с; она проверяет текущие SHA исходника/доказательств, а не заменяет CPU suite.

C89 follow-up `c7da43da` исправляет lexical binding шага/условия при continue в теле с одноимённой переменной. Заголовок сохраняет своё окружение, локальная затеняющая переменная получает отдельное HIR имя. Три конкретных случая for/do/while имеют независимое ожидаемое 809 и raw/final VM плюс девять native assertions. Root полный C89 PASS 5,556 с. Это не доказательство всей области C lexical scope.

Исходные frozen failure и donor proof сохранены в reports/c89-control-shadowing; conditional-expression работа остаётся отдельно. Полный retained pipeline29df08bb всё ещё live, итог не заявлен.
