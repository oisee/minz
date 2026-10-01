# Nanz-ротозумер для галереи — 2026-10-01

В [rotozoomer.nanz](../examples/nanz/rotozoomer.nanz) каждый экранный пиксель берёт цвет из повторяющейся текстуры 16×16. Координаты вычисляются фиксированной матрицей `[a b; -b a] / 256`; изменение `a` и `b` между кадрами меняет одновременно угол и масштаб. Внутренний цикл продвигает `u` и `v` сложениями, без умножения на каждом пикселе. Графика выводится через canvas-host MIR2 VM.

`main` строит восемь кадров 256×192 через `mzv`, а [сборщик GIF](../scripts/build_rotozoomer_gif.py) соединяет их в 14-кадровый цикл с обратным ходом. Сборщик требует Pillow. Из корня репозитория:

```sh
make -C minzc mzv
python3 scripts/build_rotozoomer_gif.py --output build/rotozoomer.gif
(cd minzc && go test ./pkg/nanz -run '^TestRotozoomerGallery$' -count=1)
```

Тест запускает тот же Nanz-файл через MIR2 VM и сравнивает пиксели кадров 0, 3 и 7 с [GIF](../media/rotozoomer.gif); дополнительно проверяет число кадров. Это доказательство текущего HIR→MIR2→VM пути. Z80 backend и скорость на настоящем Spectrum этим тестом не подтверждаются.
