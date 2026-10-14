# Один структурный спрайт — разные runtime-буферы

`hw.sprite` теперь принимает базу из переменной вызывающего кода. Маски,
графика, размеры, stride и смещения остаются известными при компиляции:

```nanz
import hw.sprite
fun draw(frame: ^u8) {
    @sprite() {
        destination "linear_relative" "frame" 8 4 10
        region 1 1 2 1
        row "f000" "05ab"
    }
}
```

Это две записи по `frame + 11` и `frame + 12`: первая сохраняет старшие четыре
бита, вторая целиком заменяет байт. Для Spectrum используется
`destination "zx_bitmap_bytes_relative" "frame"`; перестановка строк вычисляется
метафункцией заранее. На Z80 остаётся прибавление константы к runtime-базе,
а не вычисление экранного адреса по координатам. Новый режим требует pointer
primitive и совместим с `when`. Фиксированные destination продолжают работать.

## Проверяемая ссылка на caller

Новый `hir_caller_ref(name, type)` сверяет существующее объявление в scope
вызывающего кода до построения target HIR. Параметры и локальные переменные
перекрывают глобальные. Отсутствующая переменная и неверный тип отклоняются
даже тогда, когда метафункция ничего не эмитит. `hir_ref` остаётся для ссылок
на переменные, объявляемые самим генератором: эти два случая имеют разные
требования к моменту разрешения имени.

Это оказалось необходимо: прежний `hir_ref` позволял указать `ptr` для
переменной, объявленной `u16`, без проверки объявления. Новая библиотечная
runtime-база использует проверенный caller host. Общая семантика старого
builder host не изменена; он не объявляется новым строгим resolver.

## Проверки и границы

288 native запусков: четыре descriptors (linear, Spectrum строки 7/8 и 63/64,
последняя bitmap-строка), greedy/PBQP × ordinary/Grace Size/Grace Speed,
три runtime-базы, visible false/true и два полных фона памяти. Каждому запуску
соответствуют raw и optimized MIR VM проверки: ещё 576 запусков VM.
Независимый mask oracle и ZX address permutation из Antique Toy, глава 2,
сверяют все 65536 байт; native runner также проверяет return PC/SP.
Никаких исключений для stack/scratch в этой матрице не потребовалось.

Базы 0x4000, 0x6001 и 0xfffb включают перенос через 65535→0. Для последнего
CPU настроен на полные 64-КБ RAM. Первая проба с default Spectrum ROM protection
не могла писать по завёрнутым адресам; это исправление настройки проверки,
а не изменение ROM или pointer semantics. Обычный Spectrum ROM остаётся
read-only, в самом тесте не моделируются ROM execution, ULA contention или IRQ.
Caller отвечает за writable destination, не пересекающий code, active stack
и данные, которые должны пережить рисование.

Отдельные проверки покрывают diagnostics descriptor/primitive/name/type и
шесть caller-reference случаев, включая shadowing и отсутствие output.
Итоговый направленный прогон: Nanz PASS 0,039 с; pipeline PASS 10,928 с.
Он включает прежние pointer/compiled sprite matrices, исходные rejection
контракты, masked shape-composition и сохранённый текстовый baseline: ASM
прежних девяти структурных вариантов остался побайтно идентичным baseline.
Полный релизный корпус не повторялся.

Это стык выразительного DSL и runtime ABI. Нового ускорения за счёт shape-таблиц
для relative sprite или его допуска в measured CLI этим отчётом не заявлено;
проверить их совместное использование — следующий шаг. Checked memory
кандидаты уже отдельно интегрированы в main через PR #119.

Исходники, точные ASM/BIN и все логи сохранены на шаре:
`/mnt/safe/minz/grace/2026-10-14/structured-sprite-runtime-base/manifest.json`.
