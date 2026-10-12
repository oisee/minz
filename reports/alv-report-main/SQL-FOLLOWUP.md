SQLite result filtering now qualifies the validated output column with the subquery alias. This prevents SQLite double-quoted-string compatibility from treating an unknown output column as a string literal. A real SQLite regression requires an unknown-column error and proves that base LIMIT 1 precedes the LH filter (zero rows). Row scanning and terminal iteration errors are checked.

Validation: TestSQLiteFilteredQueryRealOrderedRows PASS (0.004 s); see sqlite-filter-followup.txt. No screen_report execution claim is made by this correction.
