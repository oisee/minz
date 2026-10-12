# Полный pipeline: наблюдаемые падения числового эталона

Executable1f2365be остаётся живым, PID2566814. Его лог сообщил factory/association parser reference drift: 1296385 T, 2380 B вместо фиксированных 1296873 T, 2391 B. Независимые parser state/event проверки в этих тестах стоят до cost assertion и не сообщили ошибки. Это повод исследовать ASM delta, а не автоматически менять числа.

Здесь сохранено наблюдение, не terminal результат. Root текущий source позднее получил byte-cycle/rotate policy; агент повторяет три пути с export before assertions и сверкой direct/association артефактов. Исходный полный run не перезапускается.
