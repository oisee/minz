# Интеграция137: literal-left word arithmetic

Код48e5fe6d, donor a91ea32d. В originalwidemath abs16(0x8000) выдавал0 уже в rawVM: выражение0-x получало тип узкого левого литерала. Для nonshift integer arithmetic учитывается более широкий RHS. Независимые abs_word/add_word/subtract_word boundary models проходят raw/finalVM и native во всех трёх режимах; **144cases, rootPASS0,018 с**. [Лог](root.txt), регрессия TestSignedWordLiteralLeftKeepsArithmeticWidth. Existing U16 promotion, pointer/fixed и shift правила этим patch не изменены; полнота mixedsignedness/width языковых правил здесь не заявляется.

FullNanz donorPASS12,945 с. Root corec69e350e старше этого исправления. Originalwidemath ещё не весь PASS: newtype_area(0,0) raw/finalVM0, native15104 расследуется отдельно как aggregate-address/ABI issue. Source freeze предшествует parser fix, baseline сохранён агентом; не выдаётся новый wholemodule native PASS.
