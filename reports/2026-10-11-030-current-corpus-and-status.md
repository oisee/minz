# Свежий корпус и единая сводка состояния

Повторный showcase на dd43c77e с MINZ_SHOWCASE_CHECK_KNOWN=1: **191 PASS,1FAIL,0SKIP** из192 случаев. Каждый случай имеет точный sourcepath, status и stage в docs/grace-current-corpus/cases.json. Единственный FAIL — ZVM: nativeu32 caller spill width4 и некорректные wide load/store instructions; backend исправляется отдельно. Это сборочный gate; исполнение всех191программ не заявляется.

Обновлена docs/grace-current-state.md: отражены удаление LIR, свежий корпус, HashMap training/heldout, signedfusion, Lizp full-screen и новые табличные guards. Исторические замеры остаются с прежним контекстом. Последняя опубликованная версия сайта всё ещё Sites60; новая сводка пока локальная и на шаре.

API ContractOpt=false в CompileHIRSteps игнорировался: исправление и независимые сравнения actual ABI/нативных вызовов идут отдельно. Старые измерения из buggycontext не переписываются как результаты исправленной сборки.
