# Console: явные source/ASM контракты и native proof

Исправлены два дефекта исходного ZX example/API, не семантика компилятора: `3ec12f47` закрепляет B/D/C для `con_draw_char`; `f2a4e033` передаёт `c"..."` в NUL-reading `con_print`. Все другие ASM functions stdlib/zx/console.nanz имеют соответствующие body explicit pins (zx_poke/peek/in_fe/border, screen_addr, attr_addr); con_clear аргументов не имеет. Book `docs/Nanz_Language_Book.md:215` документирует counted plain-expression literals и C strings; глава9 — explicit register contracts. Снимки исправленных sources и новые SHA в отдельном freeze.json. Исходный провал сохранён коммитом 845f3887, не переписан.

Неизменённая структура whole-main ZX demo после этих source corrections проходит ordinary/Grace Size/Grace Speed. Независимая модель 768-byte font fixture, screen bitmap+attributes целиком 4000..5aff, decimal12345/3, lowercase a и курсор11/0 совпадают. Host выдаёт сначала восемь отпущенных keyboard rows, затем FD bit0, проверяются ровно17 IN FE адресов. Actual CALL возвращается в7003 с SPf000, глубина18B. Полная RAM и profiler WriteCount разрешают только screen, три mutable cursor/attr globals и фактически использованный stack; font/literals/readonly tables/code не записаны. Это explicit ZX host model, не запуск на физическом Spectrum и не CP/M.

| Режим | BIN | T-states whole-main |
|---|---:|---:|
| ordinary | 2180 | 404452 |
| Grace Size | 2173 | 404501 |
| Grace Speed | 2173 | 404501 |

Grace здесь уменьшает7B и добавляет49T; ускорения не заявляем. Два byte-identical архивных ex18 не менялись: whole-main exact stdout/stderr и дополнительно all256 byte каждого output helper и lengths0..255 NUL-string по4000/fffe проходят три режима. Последний focused gate PASS0.538s. VM OpAsm=no-op не используется как proof. Все ASM/BIN/MIR/source/results и retained executable сохранены; общий pipeline/CLI не повторяли.

Область доказательства: оригинальные ex18 все callable functions; исправленный ZX demo whole-main и задействованные library paths, один frozen lowercase keyboard input/font fixture. Не заявляем все клавиши, scroll/input editing, все library functions или настоящую ROM font.
