# Lizp cond_test: граница языкового return контракта

Exact `examples/lizp/cond_test.lizp` сохранён без изменения. На `49888ebc` повторён существующий `TestOriginalLizpCondMissingReturnObservation`: declared `classify(n:u8)->u8` с bare cond42 возвращает **нет значений** raw/final VM, а nativeRET оставляет staleA42. Категория2, ожидаемая обычным Lisp читателем, этим компилятором не произведена. PASS observation теста не behaviorPASS программы; status должен остаться semantic-policy-gap, общая цель pending.

Прочитанная authority:

- `docs/Eight_Languages_One_Binary.md:367` называет Lizp Scheme/Lisp и показывает только **явные** `(return ...)`, включая if ветви; не задаёт implicit last-expression return и не объявляет bare expression syntax invalid.
- Bookv7 §21.2/§21.4: Lizp — thin sugar over Lanz, cond/when/setq, desugaring чистый HIR. Ни полного обязательного explicitreturn rule, ни lastvalue rule там нет. В §2.14 отдельно описан **другой** implicitreturn exception для native asm `(ret REG)`, поэтому blanket HIR missing-return rejection без анализа asm незаконен.
- `lizp.go:164` desugarDefun преобразует всеbody формы как statements. `desugarReturnValue` обрабатывает cond/progn только **в explicit return context**, иначе barecond остаётся if-statements с discarded value.
- `lizp_test.go:77 TestCompileCond` **специально принимает** declared-u8 defun с barecond. Это толькоcompile test, но существующий acceptance contract не поддерживает заявление 'source alreadyinvalid'.
- Lanz `compileFunc` задаёт RetTy и body какblock; HIRVerify проверяет ReturnStmt, а не allpathresult. Ввести широкую диагностику всех nonvoidfallthrough значило бы впервые выбрать общую языковую политику, затронуть native asm/infinitecontrolflow и другиеfrontends.

## Два конкретных решения для Alice

1. **Lizp требует explicitreturn для результата.** Пример корректен как `(defun classify ... ->u8 (return (cond ...)))`. Compiler должен reject исходныйbarecond с точной диагностикой `classify: non-void function can fall through without a result`; TestCompileCond превращается вnegativeacceptance regression. Полноценная CFG/result-admission должна сохранять infinitefunctions безfallthrough и documentedasmreturnedregister cases. Original source остаётся rejected corpus case, не молча переписанныйgreenexample.
2. **Lizp defun возвращает value последней bodyформы.** Original source должен вернуть0/1/2/255 длявсех256n; main42 должен установитьscore2 (2→when3→unless2). Lowering valuecontext нужен дляcond/if/progn/let, sideeffectprefix исполняется ровноодинраз; mixedexplicitreturn/controlflow и voidbody требуютspecifiedsemantics. Нельзя просто добавитьreturnпоследнейliteralbranch безэтого. Lanz/Nanz поведение не обязано автоматическименяться.

Оба решения разумны, ниодно не следуетнормативно изпрочитаннойдокументации. Поэтомуproductionизменений нет и sourceнеисправлен. Root подтвердил сохранениеgap, а не внедрениеpolicy. После явноговыбора нужны fulloriginalclassify0..255 +originalmainglobalorderedstate+negativepartialbranchtests, allthreeVM/native modes. ФизическийZ80 staleA не можетслужитьязыковымreturnoracle.

Локальныйлог observation иSHAsource вreports/lizp-return-policy; share `/mnt/safe/minz/grace/2026-10-11/lizp-return-policy/REPORT.md`. Полныйpipelineне запускался.
