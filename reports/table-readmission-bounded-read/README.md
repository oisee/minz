# Ограниченное чтение пула таблиц

CLI readmission читает не более1MiB+1байта из input и отвергает превышение до JSON decode. Ранее byte limit проверялся после os.ReadFile, поэтому ограничивал только допустимый JSON, но не объём чтения. Теперь фиксирован ресурс самой загрузки. Input закрывается до подготовки compiler; ошибки чтения/close возвращаются.

Разреженный64MiBфайл отвергается без output. Entryduplicatebudget и полный realimporter/compiler/all256nativeweighted replay также PASS. Fullcmd/import-enrt1,617с. Semantics/table admission не менялись; production compiler lookup не расширен. Pipeline350149ad/dd100284 продолжают выполнять свои сохранённые binaries.
