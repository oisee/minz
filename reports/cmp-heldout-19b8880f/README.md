# Heldout после byte comparison fix

Checkpoint19b8880f: повтор ранее зафиксированных parser, PFCCO, arithmetic и Pascal bubble-sort heldout, плюс12-сочетаний сохранённых weighted register assignments. Все5tests PASS34,124с. Это прежние отложенные нагрузки, не новые inputs, выбранные после результата.

Лог root.txt сохраняет measured candidates, selected decisions и heldout. Полный MIR2 этой backend-правки прошёл66,668с в cmp-live-accumulator. Живой полный pipeline350149ad не включает этот fix; его checkpoint не переименован и не объявлен новым monolithic PASS.
