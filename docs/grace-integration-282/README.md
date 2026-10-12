# Frozen literal-call test выполняется в обычном suite

TestCallLiteralTokenizerProspective больше не пропускается без MINZ_CALL_LITERAL_TOKENIZER_EXPORT. Безenv он использует t.TempDir(), выполняет те же frozen training/selection/heldout и удаляет временные exports после теста. Explicitenv по-прежнему позволяет сохранить evidence постоянно.

Root normal invocation безenv PASS2,110с, actual RUN/PASS и candidate/heldout measurements в логе. Ожидания, workloadfreeze, selectiontie и selectedBIN reuse не менялись. Compiler production не менялся. Older donor test snapshots и их manifests остаются историческими, не переписываются.

Env-audit281 описывает прежний checkpoint859e7a52; начиная с этого изменения все шесть соответствующих heldout тестов действительно исполняются в normal suite. Retainedfullpipeline1f2365be не содержит новую literal-проверку вообще.
