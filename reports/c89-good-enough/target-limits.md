# C89: предел объекта и sizeof на 16-битной цели

16 октября 2026 года. Target ABI: signed char8, short/int/pointer/size_t16,
long32, little-endian, alignment 1. `sizeof` имеет unsigned size_t.

До исправления объект из 70 000 bytes принимался, а `sizeof` превращался в
4464 после усечения. `sizeof(char[n])` принимался, но давал ноль вместо
runtime size. VLA — C99 extension, runtime поддержка здесь не реализована.

Теперь frontend задаёт parser target limit 65535 bytes и запрет VLA types.
Проверяется размер объявленных объектов, aggregate parameters, compound literals
и обоих sizeof forms, включая global constant initializers. Typedef и указатель
на крупный непрозрачный тип сами не размещают этот объект. Настройки optional:
другие пользователи parser без этих настроек сохраняют прежний допуск.
Runtime sizeof дополнительно не имеет fallback в константу ноль в lowering.

`TestC89TargetObjectLimits` проверяет global/local/static/nested/parameter и
constant-expression случаи, VLA sizeof и указатели на VLA. Две положительные
функции проверяют sizeof(char[65535]), global initializer и отсутствие эффекта
x++ внутри sizeof. Raw/final VM и Z80 default/Grace/Size: 6 native / 8 VM.
ISO C90 6.3.3.4 задаёт sizeof; это проверка выбранного target limit,
не заявление о всей области ISO C90 и не гарантия места для code/data/stack.

```sh
cd minzc
go test ./pkg/cparse ./pkg/c89 -run 'TestC89(TargetObjectLimits|SizeofTargetBoundaryVMZ80)' -count=1 -v
```

Объект из 65535 bytes оставляет мало или вовсе не оставляет памяти для
программы. Общий бюджет flat 64-KB code/data/stack остаётся обязанностью
сборки и выбранной runtime layout; тест использует размер типа, а не размещает
такой массив в native executable.
