# API повторного допуска сохранённых назначений

RegAllocTable.ReadmitCheckedCandidates рассматривает bounded пул сохранённых назначений как предложения для одной текущей LocalTableProblem. Dense позиции сопоставляются с отсортированными текущими vregs. AddChecked заново проверяет все ограничения и вычисляет стоимость; прежние Signature/Cost не служат разрешением на emission. Старая запись может предложить пригодное назначение другой операции: API не заявляет совпадение исторической формы, в JSON нет её полного графа.

Предел1..1024 задаётся явно. Переполнение пула отвергается до изменения destination; snapshots копируются под read lock и обходятся детерминированно по Signature. Stale function binding отвергается. Вызов предназначен для offline подготовки текущего сертификата; автоматический production lookup не расширен.

Native source→MIR→table→Z80 test переносит add5 из отдельного модуля в совместный caller/loops модуль, использует API и подтверждает all256inputs. Negative controls: claimed cost=-12345 пересчитывается; неверный return ABI не допускается; mutation после подготовки и превышение bound отвергаются. Полный VIR PASS2,358с; source/native PASS0,620с.

Связанные прежние доказательства: reports/table-module-readmission и reports/joint-profile-pinned-table. Ограничения leaf/table не сняты; произвольные CFG/calls/islands не разрешены. Рабочий compiler не получает зависимости от Z3.
