# Memory-host admission: unreachable extern declarations

Теперь закрытый memory-host стенд допускает неиспользуемое extern-объявление
без удаления его из исходника и без фиктивной реализации. Пример:
`02_sum_array_idiomatic.nanz` объявляет `add_to_acc(u8)->void`, но оба
полных алгоритма работают без него. Раньше admission требовал host contract
для всех extern глобально; теперь contract обязателен для direct-call closure
точных workload entry **в исходном и в compiled MIR**.

Source closure включает вызовы во всех блоках, даже `if false`: удаление
такого вызова оптимизатором не снимает host obligation. Полный исходник,
метаданные, compatibility и profile lineage не сокращаются. Любой retained
extern проверяется против source callable identity: extern flag, отсутствие
body, имена/типы/классы параметров, полный return contract, адрес и объявленные
clobbers. Действительно мёртвую декларацию можно удалить обычным DCE.

Глобальные ограничения сохранены: косвенные вызовы запрещены; взятие адреса
любой функции теперь тоже явно запрещено, чтобы callback не ускользнул из
модели direct calls. Необъявленные direct calls отвергаются и в исходнике,
и в кандидате; прежнее исключение compiler print вне reachable closure
сохранено. Registered hosts проходят прежние проверки builtin ABI.

Raw VM получает отказный hook для unbound extern (`name` и `@name`), который
не выдаёт эффектов/значения. Приватные hooks инструментирования допускаются
только через внутреннюю exact-owned карту: имя с похожим prefix не даёт
авторитета. Тест проверяет реальные независимые loop counts при collision,
а вызов пользовательского unbound pseudo-hook отвергает до запуска.
Native executable mask остаётся привязанной к reachable функциям и точным
собранным instruction extents. Проверены фактически собранные unreachable
body/stub: принудительный fetch отвергается до изменения A/SP, при этом
реальный reachable entry возвращает 173. EQU alias tests сохранены.

## Exact idiomatic source proof

`TestOriginalIdiomaticSumReadOrderVMZ80` теперь разбирает полный исходник,
добавляя только отдельный `peek`-наблюдатель. Никаких extern declarations
не удаляется; сохранены полный source, compiled-source, оба SHA и original
MIR. Независимый byte-sum/ordered-read oracle, raw/final VM, полный 514-byte
RAM/guards и native PC/SP/code проверяют оба original entry:
all256 длин × два offset × два entry = **1024 вызова** плюс observer.

Результат не изменился: **3 984 313 T**, **46 B**, BIN SHA256
`82a1e4e76f5d3deebf9f53b6045c8fec39c5168dc3b3796b47148eff679f2d66`.
Это стоимость workload в эмуляторе, не hardware/benchmark claim.
Book v7 §2.5, §5.1/§5.3 определяют array traversal и mutable capture;
countdown `range` главы6 сюда не относится.

Исторический [adapter packet](grace-original-idiomatic-sum/manifest.json)
остался побайтово неизменным. Новый packet доказывает сохранение оригинальных
объявлений; public CLI и callback runtime по-прежнему не заявлены.
Catalog остаётся **194 exact paths /36 VM-native + отдельный native-only CPM**.

## Проверки и артефакты

Full HIR: **PASS 25.997s**. Focused native target, actual dead extern fetch,
EQU alias и exact idiomatic: **PASS 3.317s**. Admission/namespace negative
packet: **PASS 0.081s**; catalog **PASS 1.414s**. Тесты проверяют direct/
transitive/constant-false reachable extern, injected candidate call,
function address/indirect escapes, unknown dead calls, body/ABI/address/
clobber mutation и source obligations после удаления candidate call.
Новые backend изменения не требовались; full slow pipeline не запускался.

[Новый manifest](grace-memory-host-dead-extern/manifest.json) содержит
source/provenance, workloads, независимый model, observations, MIR/ASM/BIN
и logs. Крупные JSON/MIR gzip сохранены без изменений. Development-first
log показывает обнаруженный конфликт со старым prefix-only ожиданием теста;
финальные full/negative logs подтверждают исправленную exact-owned semantics.
