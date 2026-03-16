REPORT zguessing.

* Number guessing game — ABAP style.
* No SE80, no transport request, just pure Z80 vibes.

DATA: lv_secret TYPE i VALUE 42,
      lv_guess  TYPE i VALUE 0,
      lv_tries  TYPE i VALUE 0,
      lv_diff   TYPE i.

WRITE 'Guess the number (1-100)!'.

DO 7 TIMES.
  lv_tries = lv_tries + 1.

  * In real Z80 this would read keyboard input
  * For now we simulate: try 50, 25, 37, 43, 40, 41, 42
  CASE lv_tries.
    WHEN 1. lv_guess = 50.
    WHEN 2. lv_guess = 25.
    WHEN 3. lv_guess = 37.
    WHEN 4. lv_guess = 43.
    WHEN 5. lv_guess = 40.
    WHEN 6. lv_guess = 41.
    WHEN 7. lv_guess = 42.
  ENDCASE.

  IF lv_guess = lv_secret.
    WRITE 'Correct!'.
    WRITE lv_tries.
    EXIT.
  ELSEIF lv_guess < lv_secret.
    WRITE 'Too low!'.
  ELSE.
    WRITE 'Too high!'.
  ENDIF.
ENDDO.
