REPORT zabap_on_x86.

* ABAP running natively on x86_64 — not SAP, not Z80!
*
* Try:
*   mz abap_on_x86.abap --emit llvm -o abap.ll  (see LLVM IR from ABAP)
*   clang abap.ll -o abap && ./abap              (native x86_64 binary!)
*   mz abap_on_x86.abap --emit mir2              (see SSA intermediate)
*   mzn abap_on_x86.abap                         (native via QBE)
*   mzv abap_on_x86.abap                         (MIR2 VM)

DATA: lv_a TYPE i VALUE 0,
      lv_b TYPE i VALUE 1,
      lv_n TYPE i VALUE 10,
      lv_t TYPE i,
      lv_i TYPE i VALUE 0.

WHILE lv_i < lv_n.
  lv_t = lv_a + lv_b.
  lv_a = lv_b.
  lv_b = lv_t.
  lv_i = lv_i + 1.
ENDWHILE.

* lv_a should be 55 (10th fibonacci)
