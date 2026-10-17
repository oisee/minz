REPORT zledger.
TYPES: BEGIN OF ty_entry,
         code   TYPE c LENGTH 3,
         amount TYPE i,
       END OF ty_entry.
DATA entry TYPE ty_entry.
DATA probe TYPE ty_entry.
DATA ledger TYPE STANDARD TABLE OF ty_entry WITH EMPTY KEY INITIAL SIZE 6.
DATA x TYPE i.
DATA y TYPE i.
DATA seen TYPE i.
DATA kept TYPE i.

FORM find_first_negative.
  LOOP AT ledger INTO probe.
    IF probe-amount < 0.
      y = sy-tabix.
      RETURN.
    ENDIF.
  ENDLOOP.
  y = 77.
ENDFORM.

entry-code = 'ALPHA'.
entry-amount = 10.
APPEND entry TO ledger.
entry-code = 'B'.
entry-amount = -3.
APPEND entry TO ledger.
APPEND INITIAL LINE TO ledger.
seen = sy-tabix.

LOOP AT ledger INTO entry.
  entry-amount = entry-amount + 100.
  IF sy-tabix = 1.
    entry-code = 'GEN'.
    entry-amount = 7.
    APPEND entry TO ledger.
  ENDIF.
  IF entry-code = 'B'.
    CONTINUE.
  ENDIF.
  x = x + entry-amount.
  DO 2 TIMES.
    x = x + sy-index * sy-tabix.
  ENDDO.
  WRITE |{ entry-code }:{ entry-amount };|.
ENDLOOP.
seen = seen * 10 + sy-tabix.

READ TABLE ledger INTO probe INDEX 1.
kept = probe-amount.
READ TABLE ledger INTO probe INDEX 4.
kept = kept * 1000 + probe-amount.

LOOP AT ledger INTO entry.
  PERFORM find_first_negative.
  seen = seen * 10 + sy-tabix.
  IF sy-tabix = 2.
    EXIT.
  ENDIF.
ENDLOOP.
seen = seen * 10 + sy-subrc.
seen = seen * 10 + sy-tabix.

probe = entry.
CLEAR probe-amount.
REFRESH ledger.
y = y * 10 + lines( ledger ).
APPEND probe TO ledger.
APPEND entry TO ledger.
y = y * 10 + lines( ledger ).
