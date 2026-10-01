# Проверка изображений на фасаде MinZ — 2026-10-01

Проверены три прежние карточки `README.md` на текущем `main` и заменены на изображения, которые можно заново получить из нынешних исходников. Проверка разделяет компиляцию, выполнение MIR2 VM и исполнение Z80-бинаря: одно не доказывает другое.

## Прежние карточки

| Карточка | Что удалось подтвердить | Вывод |
| --- | --- | --- |
| ObjC `plasma.m` | `TestPlasmaRender` вызывает `Plasma_render` в MIR2 VM; новый PNG совпал с `media/plasma.png` по декодированным пикселям. Прямой запуск `mzv --headless examples/objc/plasma.m` завершается `function @main not found`: файл содержит эффекты, но не entry point. | Изображение настоящее; для воспроизведения нужен тестовый harness. |
| `zsql_mara_zx.abap` | `mzv --headless` печатает текст с командами SQL, но исходник содержит только `WRITE` заранее записанного транскрипта. Текущий Z80-прогон через `mze --profile` дал повреждённый экран, отличный от `media/zsql_mara_zx_spectrum.png`. Настоящий клиент `examples/nanz/zsql.nanz` сейчас не проходит сборку из-за неопределённых `sql__sqlite__*` меток. | Картинка не доказывает работу SQL или воспроизводимый экран; убрана с фасада. |
| `media/mzv_sphere_minz.png` → `fun/raymarcher.nanz` | PNG относится к старому `.minz`-пути и старой форме CLI. Текущий `mzv --headless fun/raymarcher.nanz` завершается `unknown function normalize`; Z80-ассемблер тоже отвергает его текущий output. | Связь картинки с указанным текущим исходником не подтверждена; убрана с фасада. |

## Нынешняя галерея

`media/plasma.png` и `media/xor.png` воспроизводятся из `examples/objc/plasma.m` через `TestPlasmaRender`. `media/canvas_house.png` воспроизводится из `examples/nanz/canvas_house.nanz`: исходник непосредственно запускается через `mzv --headless`, а `TestCanvasImplShowcase` читает тот же файл. Оба теста сравнивают декодированные пиксели с PNG в репозитории, а не только успешность исполнения.

Из корня репозитория:

```sh
make -C minzc mzv
mkdir -p build
minzc/mzv --headless examples/nanz/canvas_house.nanz
cmp build/canvas_house.png media/canvas_house.png
(cd minzc && go test ./pkg/c89 ./pkg/nanz -run '^(TestPlasmaRender|TestCanvasImplShowcase)$' -count=1)
```

Отдельный quick start в корневом README компилирует и исполняет `examples/nanz/hello_cpm.nanz` на Z80-эмуляторе и печатает `Hello!`. Галерея подтверждает возможности MIR2 VM и двух frontend, а не готовность этих графических программ к ZX Spectrum backend. Для `fun/` есть отдельный [build audit](../fun/README.md).
