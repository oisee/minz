REPORT zbubble.

* Bubble sort in ABAP — the algorithm your ABAP teacher
* told you never to use in production.
* But on Z80 with 64KB RAM, who's counting?

DATA: lv_size   TYPE i VALUE 8,
      lv_i      TYPE i,
      lv_j      TYPE i,
      lv_temp   TYPE i,
      lv_swap   TYPE i,
      lv_limit  TYPE i.

* Sort 8 values using registers as "array"
* (Real arrays need LOOP AT — coming in Phase 2!)
DATA: lv_0 TYPE i VALUE 64,
      lv_1 TYPE i VALUE 25,
      lv_2 TYPE i VALUE 12,
      lv_3 TYPE i VALUE 22,
      lv_4 TYPE i VALUE 11,
      lv_5 TYPE i VALUE 90,
      lv_6 TYPE i VALUE 33,
      lv_7 TYPE i VALUE 7.

WRITE 'Before:'.
WRITE: lv_0, lv_1, lv_2, lv_3, lv_4, lv_5, lv_6, lv_7.

* Bubble sort passes
lv_i = 0.
WHILE lv_i < 7.
  lv_j = 0.
  lv_limit = 7 - lv_i.
  WHILE lv_j < lv_limit.
    * Compare adjacent pairs using nested IFs
    * (ABAP on Z80 doesn't have internal tables yet!)
    CASE lv_j.
      WHEN 0. IF lv_0 > lv_1. lv_temp = lv_0. lv_0 = lv_1. lv_1 = lv_temp. ENDIF.
      WHEN 1. IF lv_1 > lv_2. lv_temp = lv_1. lv_1 = lv_2. lv_2 = lv_temp. ENDIF.
      WHEN 2. IF lv_2 > lv_3. lv_temp = lv_2. lv_2 = lv_3. lv_3 = lv_temp. ENDIF.
      WHEN 3. IF lv_3 > lv_4. lv_temp = lv_3. lv_3 = lv_4. lv_4 = lv_temp. ENDIF.
      WHEN 4. IF lv_4 > lv_5. lv_temp = lv_4. lv_4 = lv_5. lv_5 = lv_temp. ENDIF.
      WHEN 5. IF lv_5 > lv_6. lv_temp = lv_5. lv_5 = lv_6. lv_6 = lv_temp. ENDIF.
      WHEN 6. IF lv_6 > lv_7. lv_temp = lv_6. lv_6 = lv_7. lv_7 = lv_temp. ENDIF.
    ENDCASE.
    lv_j = lv_j + 1.
  ENDWHILE.
  lv_i = lv_i + 1.
ENDWHILE.

WRITE 'After:'.
WRITE: lv_0, lv_1, lv_2, lv_3, lv_4, lv_5, lv_6, lv_7.
