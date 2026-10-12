# Контекст импорта в историческом сравнении фронтендов

TestGrace_AllFrontends и TestGraceConvergence теперь разбирают Nanz с BaseDir исходников и StdlibDir; PL/M выбирается из examples/plm вместо пустого examples/nanz/*.plm. Parser failures показывают причину. Это устраняет искусственный parseFAIL на импортирующих примерах, без изменения frontend/compiler.

Сборочное сравнение137файлов: Nanz72,Lanz1,Lizp14,PLM5,Pascal11,C89 34. Все137компилируются в обоих режимах; failures0, identical23, ASMdiverged114. TestGrace_AllFrontends PASS10,81с, convergence diagnostic PASS9,32с; совместно20,147с. Отличие ASM не считается semantic mismatch и identicalASM не заменяет independent native oracle. Глобальный каталог194путей остаётся отдельным gate.

Полныйpipeline350149ad запущен до этой test-only правки и продолжает работать; его диагностические parseFAIL относятся к прежнему окружению сравнения. Retaineddd100284 не остановлен. Новый результат сохраняет полноту выбранных каталогов, но не добавляет runtimecoverage137программ.
